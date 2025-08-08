#!/bin/sh
set -e

# Ensure base server directory exists
mkdir -p /marchat/server

# Ensure config directory exists inside server directory
mkdir -p /marchat/server/config

# Ensure db directory exists inside server directory
mkdir -p /marchat/server/db

# Ensure data directory exists inside server directory
mkdir -p /marchat/server/data

# Ensure plugins directory exists inside server directory
mkdir -p /marchat/server/plugins

# Fix ownership if we have write access
if [ -w "/marchat/server" ]; then
    chown -R marchat:marchat /marchat/server 2>/dev/null || true
fi

# Execute the main application
exec ./marchat-server "$@"
