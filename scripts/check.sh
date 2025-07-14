#!/bin/bash
set -e

echo "=== Checking for package updates ==="

mkdir -p "/app/cache"

CHECKSUMS_FILE="/app/cache/package_checksums.txt"
REBUILD_TRIGGER_FILE="/app/rebuild_trigger.txt"
REMOTE_PACKAGES_FILE="/app/remote_packages.json"

declare -A old_checksums
if [ -f "$CHECKSUMS_FILE" ]; then
    echo "Loading existing checksums from '$CHECKSUMS_FILE'..."
    while IFS='|' read -r filename checksum; do
        [[ -n "$filename" && -n "$checksum" ]] && old_checksums["$filename"]="$checksum"
    done < "$CHECKSUMS_FILE"
else
    echo "No existing checksums file found. All packages will be treated as new."
fi

PROJECT_IDS=$(grep -vE '^\s*#|^\s*$' "/app/packages.txt" | awk '{print $1}' | tr '\n' ' ')
echo "Processing Project IDs: ${PROJECT_IDS}"

remote_packages_json=""
temp_json_file=$(mktemp)

for project_id in ${PROJECT_IDS}; do
    echo "Fetching releases for project ${project_id}..."
    releases_url="https://gitlab.com/api/v4/projects/${project_id}/releases"
    
    latest_release=$(curl --silent --show-error --fail \
      --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
      "${releases_url}" | jq -r '.[0] // empty')
    
    if [ -z "$latest_release" ]; then
        echo "  -> No releases found for project ${project_id}, skipping."
        continue
    fi
    
    echo "${latest_release}" | jq -c '.assets.links[]? | select(.name | endswith(".pkg.tar.zst")) | {(.name): .url}' >> "$temp_json_file"
done

if [ -s "$temp_json_file" ]; then
    remote_packages_json=$(cat "$temp_json_file" | jq -s 'add')
else
    remote_packages_json="{}"
fi

rm -f "$temp_json_file"

if [ -z "$remote_packages_json" ] || [ "$remote_packages_json" == "null" ]; then
    echo "No remote packages found across all projects. Nothing to do."
    echo "{}" > "$REMOTE_PACKAGES_FILE"
    echo "false" > "/app/rebuild_needed.txt"
    exit 0
fi

echo "${remote_packages_json}" > "$REMOTE_PACKAGES_FILE"
echo "Found a total of $(echo "$remote_packages_json" | jq 'length') remote packages."

echo "=== Verifying package checksums ==="
changed_packages=()
 lucha_needed=false

echo "$remote_packages_json" | jq -r 'to_entries[] | "\(.key)|\(.value)"' | while IFS='|' read -r pkg_filename download_url; do
    echo "  Checking: ${pkg_filename}"
    temp_file=$(mktemp)
    
    if ! curl --location --silent --show-error --fail \
      --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
      --output "$temp_file" "$download_url"; then
        echo "    ✗ ERROR: Failed to download ${pkg_filename} for checksum verification. Skipping."
        rm -f "$temp_file"
        continue
    fi
    
    checksum=$(sha256sum "$temp_file" | awk '{print $1}')
    rm "$temp_file"
    
    echo "${pkg_filename}|${checksum}" >> "${CHECKSUMS_FILE}.new"
    
    if [[ ! -v old_checksums["$pkg_filename"] ]] || [[ "${old_checksums[$pkg_filename]}" != "$checksum" ]]; then
        echo "    🔄 CHANGED (new checksum: ${checksum})"
        echo "$pkg_filename" >> "$REBUILD_TRIGGER_FILE"
        rebuild_needed=true
    else
        echo "    ✓ Unchanged"
    fi
done

if [ -f "${CHECKSUMS_FILE}.new" ]; then
    mv "${CHECKSUMS_FILE}.new" "$CHECKSUMS_FILE"
fi

if [ "$rebuild_needed" = true ] || [ "$FORCE_REBUILD" == "true" ]; then
    echo "=== Rebuild needed. ==-validator.js"
    echo "true" > "/app/rebuild_needed.txt"
else
    echo "=== No package changes detected. ==="
    echo "false" > "/app/rebuild_needed.txt"
    > "$REBUILD_TRIGGER_FILE"
fi
