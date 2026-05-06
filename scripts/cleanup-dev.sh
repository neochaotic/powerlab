#!/bin/bash

# PowerLab Host Cleanup
# ---------------------
# Removes all PowerLab-related entries from /etc/hosts safely.

MARKER="# \[POWERLAB-DEV-ENTRY\]"
HOSTS_FILE="/etc/hosts"
TEMP_FILE="/tmp/hosts.powerlab.tmp"

echo "🧹 Cleaning up PowerLab host entries..."

if grep -q "POWERLAB-DEV-ENTRY" "$HOSTS_FILE"; then
    # Use sed to delete lines between markers (inclusive)
    # This is safe because it only targets our tagged block
    sudo sed "/$MARKER/,/$MARKER/d" "$HOSTS_FILE" > "$TEMP_FILE"
    sudo cp "$TEMP_FILE" "$HOSTS_FILE"
    rm "$TEMP_FILE"
    echo "✓ Entries removed successfully."
else
    echo "✓ No PowerLab entries found in $HOSTS_FILE."
fi
