#!/bin/bash
##
# deprecate-images.sh
# Tags older images in GCR/AR as deprecated, keeping only the latest.
##

set -euo pipefail

# Default to dry run unless explicitly disabled
DRY_RUN=${DRY_RUN:-true}
IMAGE_URL=${1:-}

if [[ -z "$IMAGE_URL" ]]; then
  echo "Usage: [DRY_RUN=false] $0 <image_url>"
  exit 1
fi

if [[ "$DRY_RUN" == "true" ]]; then
  echo "--- DRY RUN MODE ENABLED (No changes will be made) ---"
fi

echo "Scanning $IMAGE_URL for older versions..."

# Get all SHAs, sorted by upload time descending, skipping the first (latest) one.
OLD_SHAS=$(gcrane ls "$IMAGE_URL" --json | jq -r '.manifest | to_entries | sort_by(.value.timeUploadedMs | tonumber) | reverse | .[1:] | .[].key' || echo "")

if [[ -z "$OLD_SHAS" ]]; then
  echo "No older images found for $IMAGE_URL. Skipping."
  exit 0
fi

count=0
for sha_with_prefix in $OLD_SHAS; do
  FULL_SHA=${sha_with_prefix#sha256:}
  TAG="deprecated-public-image-$FULL_SHA"
  
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[DRY-RUN] Would tag $IMAGE_URL@$sha_with_prefix as $TAG"
    count=$((count + 1))
  else
    echo "Tagging $IMAGE_URL@$sha_with_prefix as $TAG"
    gcrane tag "$IMAGE_URL@$sha_with_prefix" "$TAG"
    count=$((count + 1))
  fi
done

echo "---"
if [[ "$DRY_RUN" == "true" ]]; then
  echo "Summary: $count images would have been tagged as deprecated."
else
  echo "Summary: Successfully tagged $count images as deprecated."
fi