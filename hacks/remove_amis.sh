#!/bin/bash

# A script to find and delete an AMI and its associated snapshots from all AWS regions.
# Usage: ./remove-ami-all-regions.sh <ami_name_or_id>

set -eo pipefail # Exit on error and on pipe failures

# --- Configuration & Pre-flight Checks ---

# The AMI name or ID to search for and delete
AMI_IDENTIFIER="$1"

# Check for required tools
if ! command -v aws &> /dev/null; then
    echo "Error: 'aws' CLI is not installed or not in your PATH." >&2
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "Error: 'jq' is not installed. Please install it to parse JSON output." >&2
    exit 1
fi

# Check for script argument
if [[ -z "$AMI_IDENTIFIER" ]]; then
    echo "Usage: $0 <ami_name_or_id>" >&2
    echo "Example (by ID):   $0 ami-0123456789abcdef0"
    echo "Example (by Name): $0 \"My App AMI v1.2\""
    exit 1
fi

echo "✅ Pre-flight checks passed. Starting the process..."

# --- Main Logic ---

# Get a list of all enabled AWS regions
REGIONS=$(aws ec2 describe-regions --query "Regions[?OptInStatus=='opt-in-not-required' || OptInStatus=='opted-in'].RegionName" --output text)
if [[ -z "$REGIONS" ]]; then
    echo "Error: Could not retrieve AWS regions." >&2
    exit 1
fi

echo "🌎 Will scan the following regions: $REGIONS"
echo "------------------------------------------------------------"

# Determine if the identifier is an AMI ID or a Name
if [[ "$AMI_IDENTIFIER" == ami-* ]]; then
    FILTER_NAME="image-id"
else
    FILTER_NAME="name"
fi
echo "🔍 Searching for AMIs with ${FILTER_NAME} matching '${AMI_IDENTIFIER}'..."
echo ""

DELETED_COUNT=0

# Loop through each region to find and delete the AMI
for REGION in $REGIONS; do
    echo "--- Processing Region: $REGION ---"

    # Find the image and its snapshots in the current region
    # We use jq to extract the ImageId and an array of SnapshotIds
    IMAGE_DATA=$(aws ec2 describe-images \
        --region "$REGION" \
        --filters "Name=$FILTER_NAME,Values=$AMI_IDENTIFIER" \
        --query 'Images[0].{ImageId:ImageId, Snapshots:BlockDeviceMappings[].Ebs.SnapshotId}' \
        --output json)

    IMAGE_ID=$(echo "$IMAGE_DATA" | jq -r '.ImageId')

    # Check if the AMI was found
    if [[ "$IMAGE_ID" == "null" || -z "$IMAGE_ID" ]]; then
        echo "No matching AMI found in $REGION. Skipping."
        continue
    fi

    echo "✅ Found AMI in $REGION: $IMAGE_ID"

    # 1. Deregister the AMI
    echo "Deregistering $IMAGE_ID..."
    if aws ec2 deregister-image --region "$REGION" --image-id "$IMAGE_ID"; then
        echo "✔️ Successfully deregistered $IMAGE_ID."
        ((DELETED_COUNT++))
    else
        echo "❌ Failed to deregister $IMAGE_ID in $REGION. Check permissions or dependencies." >&2
        continue # Skip to the next region if deregistration fails
    fi

    # 2. Delete the associated snapshots
    SNAPSHOT_IDS=$(echo "$IMAGE_DATA" | jq -r '.Snapshots[]?')
    if [[ -n "$SNAPSHOT_IDS" ]]; then
        for SNAP_ID in $SNAPSHOT_IDS; do
            echo "Deleting associated snapshot $SNAP_ID..."
            if aws ec2 delete-snapshot --region "$REGION" --snapshot-id "$SNAP_ID"; then
                echo "✔️ Successfully deleted snapshot $SNAP_ID."
            else
                echo "⚠️ Failed to delete snapshot $SNAP_ID in $REGION. It may need to be removed manually." >&2
            fi
        done
    else
        echo "No associated snapshots found to delete."
    fi

done

echo "------------------------------------------------------------"
if [[ $DELETED_COUNT -gt 0 ]]; then
    echo "🎉 Process complete. Deregistered $DELETED_COUNT AMI(s) and attempted to delete their snapshots."
else
    echo "🤷 Process complete. No AMIs matching '$AMI_IDENTIFIER' were found and deleted."
fi
