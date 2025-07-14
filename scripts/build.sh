#!/bin/bash
set -e

echo "=== Building Repository ==="

REMOTE_PACKAGES_FILE="/app/remote_packages.json"
REPO_ARCH_DIR="/app/public/x86_64"
CACHE_DIR="/app/cache"
REBUILD_TRIGGER_FILE="/app/rebuild_trigger.txt"

if [ ! -f "$REMOTE_PACKAGES_FILE" ]; then
    echo "ERROR: Remote packages file '$REMOTE_PACKAGES_FILE' not found. Cannot proceed."
    exit 1
fi

mkdir -p "$REPO_ARCH_DIR" "$CACHE_DIR"

declare -A packages_to_rebuild
if [ -f "$REBUILD_TRIGGER_FILE" ]; then
    while IFS= read -r pkg_filename; do
        [[ -n "$pkg_filename" ]] && packages_to_rebuild["$pkg_filename"]=1
    done < "$REBUILD_TRIGGER_FILE"
fi

echo "Reading package list from '$REMOTE_PACKAGES_FILE'..."
successful_downloads=0
failed_downloads=0

jq -r 'to_entries[] | "\(.key)|\(.value)"' "$REMOTE_PACKAGES_FILE" | while IFS='|' read -r pkg_filename download_url; do
    cached_file="${CACHE_DIR}/${pkg_filename}"
    repo_file="${REPO_ARCH_DIR}/${pkg_filename}"
    
    if [[ -v packages_to_rebuild["$pkg_filename"] ]] || [ "$FORCE_REBUILD" == "true" ] || [ ! -f "$cached_file" ]; then
        if [[ ! -f "$cached_file" ]]; then
            echo "  ⬇️ Downloading new/missing package: ${pkg_filename}"
        else
            echo "  🔄 Re-downloading changed package: ${pkg_filename}"
        fi
        
        if curl --location --silent --show-error --fail \
          --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
          --output "$repo_file" "$download_url"; then
            echo "    ✓ Downloaded successfully. Updating cache."
            cp "$repo_file" "$cached_file"
            successful_downloads=$((successful_downloads + 1))
        else
            echo "    ✗ ERROR: Failed to download ${pkg_filename}!"
            failed_downloads=$((failed_downloads + 1))
        fi
    else
        echo "  📋 Using cached version of: ${pkg_filename}"
        cp "$cached_file" "$repo_file"
    fi
done

echo "=== Cleaning up obsolete packages ==="
declare -A remote_packages_lookup
jq -r 'keys[]' "$REMOTE_PACKAGES_FILE" | while IFS= read -r pkg_name; do
    remote_packages_lookup["$pkg_name"]=1
done

for cached_file in "${CACHE_DIR}"/*.pkg.tar.zst; do
    [ -f "$cached_file" ] || continue
    cached_filename=$(basename "$cached_file")
    if [[ ! -v remote_packages_lookup["$cached_filename"] ]]; then
        echo "  🗑️ Removing obsolete cached package: ${cached_filename}"
        rm "$cached_file"
    fi
done

echo "=== Creating repository database in '${REPO_ARCH_DIR}' ==="
cd "$REPO_ARCH_DIR"
rm -f prismlinux.db* prismlinux.files*

package_count=$(ls -1 ./*.pkg.tar.zst 2>/dev/null | wc -l)
if [ "$package_count" -gt 0 ]; then
    echo "Building repository database with ${package_count} packages..."
    repo-add prismlinux.db.tar.gz *.pkg.tar.zst
    ln -sf prismlinux.db.tar.gz prismlinux.db
    ln -sf prismlinux.files.tar.gz prismlinux.files
    echo "✓ Repository database created successfully."
else
    echo "⚠️ No packages found. Creating an empty repository database."
    touch prismlinux.db.tar.gz prismlinux.files.tar.gz
    ln -sf prismlinux.db.tar.gz prismlinux.db
    ln -sf prismlinux.files.tar.gz prismlinux.files
fi

echo "=== Build Summary ==="
echo "Successful downloads: $successful_downloads"
echo "Failed downloads: $failed_downloads"
echo "Total packages in repo: $package_count"

if [ $failed_downloads -gt 0 ]; then
    echo "Build failed due to download errors."
    exit 1
fi
