#!/bin/bash
set -e

# Script to change the swoops-agent user on macOS
# This updates the launchd plist to run as the current user

CURRENT_USER=$(whoami)
PLIST_FILE="$HOME/Library/LaunchAgents/com.swoops.agent.plist"

echo "Fixing swoops-agent to run as: $CURRENT_USER"
echo

# Check if plist exists
if [ ! -f "$PLIST_FILE" ]; then
    echo "Error: $PLIST_FILE not found"
    echo "The agent may not be installed, or it may be installed in a different location"
    exit 1
fi

# Stop the agent if it's running
if launchctl list | grep -q com.swoops.agent 2>/dev/null; then
    echo "Stopping swoops-agent..."
    launchctl unload "$PLIST_FILE" 2>/dev/null || true
    sleep 1
fi

# Ensure /opt/swoops exists and is owned by current user
if [ -d "/opt/swoops" ]; then
    echo "Updating /opt/swoops ownership to $CURRENT_USER..."
    sudo chown -R "$CURRENT_USER:staff" /opt/swoops
else
    echo "Creating /opt/swoops owned by $CURRENT_USER..."
    sudo mkdir -p /opt/swoops
    sudo chown -R "$CURRENT_USER:staff" /opt/swoops
fi

# Update or verify plist - it should already be in the user's LaunchAgents
# so it will run as the current user by default on macOS

echo "✓ Directory permissions updated"
echo "✓ Agent plist is in $PLIST_FILE (runs as current user by default)"
echo

# Start the agent
echo "Starting swoops-agent..."
launchctl load "$PLIST_FILE"

echo
echo "✓ Done! Agent is now running as $CURRENT_USER"
echo
echo "To check status: launchctl list | grep swoops"
echo "To view logs: tail -f $HOME/Library/Logs/swoops-agent.log"
