#!/bin/bash
set -e

echo "=== Preparing Website for Deployment ==="

REPO_ARCH_DIR="/app/public/x86_64"
API_DIR="/app/public/api"
mkdir -p "$API_DIR"

# Generate API files
echo "Generating API file: packages.json"
packages_json="[]"
if [ -d "$REPO_ARCH_DIR" ] && [ -n "$(ls -A "$REPO_ARCH_DIR"/*.pkg.tar.zst 2>/dev/null)" ]; then
    temp_packages_file=$(mktemp)
    for pkg_file in "$REPO_ARCH_DIR"/*.pkg.tar.zst; do
        [ -f "$pkg_file" ] || continue

        pkg_info=$(pacman -Qip "$pkg_file" 2>/dev/null || echo "")
        if [ -n "$pkg_info" ]; then
            name=$(echo "$pkg_info" | grep -m1 "^Name" | sed 's/Name\s*:\s*//')
            version=$(echo "$pkg_info" | grep -m1 "^Version" | sed 's/Version\s*:\s*//')
            desc=$(echo "$pkg_info" | grep -m1 "^Description" | sed 's/Description\s*:\s*//')
            arch=$(echo "$pkg_info" | grep -m1 "^Architecture" | sed 's/Architecture\s*:\s*//')
            filename=$(basename "$pkg_file")
            size=$(ls -lh "$pkg_file" | awk '{print $5}')
            modified=$(date -r "$pkg_file" --iso-8601=seconds)
            depends=$(echo "$pkg_info" | grep -m1 "^Depends On" | sed 's/Depends On\s*:\s*//' || echo "None")
            url=$(echo "$pkg_info" | grep -m1 "^URL" | sed 's/URL\s*:\s*//' || echo "None")
            license=$(echo "$pkg_info" | grep -m1 "^Licenses" | sed 's/Licenses\s*:\s*//' || echo "None")

            jq -cn \
              --arg name "$name" \
              --arg version "$version" \
              --arg desc "$desc" \
              --arg arch "$arch" \
              --arg filename "$filename" \
              --arg size "$size" \
              --arg modified "$modified" \
              --arg depends "$depends" \
              --arg url "$url" \
              --arg license "$license" \
              '{name: $name, version: $version, description: $desc, architecture: $arch, filename: $filename, size: $size, modified: $modified, depends: $depends, url: $url, license: $license}' >> "$temp_packages_file"
        fi
    done

    if [ -s "$temp_packages_file" ]; then
        packages_json=$(cat "$temp_packages_file" | jq -s .)
    fi
    rm -f "$temp_packages_file"
fi

echo "$packages_json" > "${API_DIR}/packages.json"

echo "Generating API file: stats.json"
if [ -d "$REPO_ARCH_DIR" ]; then
    PACKAGE_COUNT=$(ls -1 "$REPO_ARCH_DIR"/*.pkg.tar.zst 2>/dev/null | wc -l || echo 0)
    REPO_SIZE=$(du -sh "$REPO_ARCH_DIR" 2>/dev/null | awk '{print $1}' || echo '0B')
else
    PACKAGE_COUNT=0
    REPO_SIZE="0B"
fi

jq -n \
  --arg total_packages "${PACKAGE_COUNT}" \
  --arg repository_size "${REPO_SIZE}" \
  --arg last_updated "$(date --iso-8601=seconds)" \
  --arg arch "x86_64" \
  --arg repo_name "prismlinux" \
  '{total_packages: ($total_packages | tonumber), repository_size: $repository_size, last_updated: $last_updated, architecture: $arch, repository_name: $repo_name}' > "${API_DIR}/stats.json"

# If there is a custom frontend, build it and copy to public/
if [ -d "/app/website" ]; then
    echo "Found 'website' directory. Building custom frontend..."
    cd /app/website

    if [ -f "package.json" ]; then
        echo "Installing dependencies and building with Bun..."
        export PATH="/root/.bun/bin:$PATH"
        bun install

        if grep -q '"build"' package.json; then
            bun run build
        fi

        echo "Copying build output to '/app/public/'..."
        if [ -d "dist" ]; then
            rsync -a "dist/" "/app/public/"
        elif [ -d "build" ]; then
            rsync -a "build/" "/app/public/"
        elif [ -d "out" ]; then
            rsync -a "out/" "/app/public/"
        else
            echo "Warning: No build output found!"
        fi
    fi
    cd /app
else
    echo "No custom 'website' directory. Generating a simple default index.html..."
    echo "<!DOCTYPE html><html><body><h1>Default index</h1></body></html>" > /app/public/index.html
fi

echo "=== Final public/ directory structure ==="
find /app/public -type f
