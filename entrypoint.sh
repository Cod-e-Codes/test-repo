#!/bin/sh
set -e

# Ensure config directory has correct ownership
if [ -d "/marchat/config" ]; then
    # Fix ownership if needed (only if we have write access)
    if [ -w "/marchat/config" ]; then
        chown -R marchat:marchat /marchat/config 2>/dev/null || true
    fi
fi

mkdir /marchat/config/db/

# Execute the main application
exec ./marchat-server "$@"