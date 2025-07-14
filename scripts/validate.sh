#!/bin/bash
set -e

echo "Validating configuration..."

if [ ! -f "/app/packages.txt" ]; then
    echo "ERROR: Package configuration file 'packages.txt' not found!"
    exit 1
fi

if [ -z "$GITLAB_TOKEN" ]; then
    echo "ERROR: Environment variable 'GITLAB_TOKEN' is not set! It's required to read project releases."
    exit 1
fi

echo "Checking 'packages.txt' format..."
while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]]; then
        continue
    fi
    
    project_id=$(echo "$line" | awk '{print $1}')
    if ! [[ "$project_id" =~ ^[0-9]+$ ]]; then
        echo "ERROR: Invalid format in 'packages.txt'. Line should contain a numeric Project ID: '$line'"
        exit 1
    fi
    echo "  ✓ Valid Project ID: $project_id"
done < "/app/packages.txt"

echo "Configuration validation passed!"
