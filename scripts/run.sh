#!/bin/bash
set -e

echo "Starting repository build process..."
/app/scripts/validate.sh
/app/scripts/check.sh
if [ -f /app/rebuild_needed.txt ] && [ "$(cat /app/rebuild_needed.txt)" = "true" ] || [ "$FORCE_REBUILD" = "true" ]; then
  /app/scripts/build.sh
fi
/app/scripts/deploy.sh
echo "Process completed."
