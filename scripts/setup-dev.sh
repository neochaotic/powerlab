#!/bin/bash

# PowerLab Safe Development Domain Setup
# --------------------------------------
# This script is designed to be non-destructive.
# 1. It creates a backup of your /etc/hosts.
# 2. It only appends if the entry doesn't exist.
# 3. It uses clear markers for easy removal.

DOMAIN="powerlab.test"
IP="127.0.0.1"
MARKER="# [POWERLAB-DEV-ENTRY]"
HOSTS_FILE="/etc/hosts"
BACKUP_FILE="/etc/hosts.powerlab.bak"

echo "🛡️ PowerLab Safe Setup starting..."

# 1. Create backup
if [ ! -f "$BACKUP_FILE" ]; then
    echo "Creating backup at $BACKUP_FILE..."
    sudo cp "$HOSTS_FILE" "$BACKUP_FILE"
fi

# 2. Check if domain already mapped
if grep -q "$DOMAIN" "$HOSTS_FILE"; then
    echo "✓ $DOMAIN is already configured in $HOSTS_FILE"
else
    echo "Adding $DOMAIN to $HOSTS_FILE..."
    # Append with markers for safety
    echo -e "\n$MARKER\n$IP $DOMAIN\n$MARKER" | sudo tee -a "$HOSTS_FILE" > /dev/null
    echo "✓ Success! $DOMAIN now points to $IP"
fi

echo "🚀 You can now use: http://$DOMAIN:5173"
echo "💡 To undo these changes, you can restore from $BACKUP_FILE or run the cleanup script."
