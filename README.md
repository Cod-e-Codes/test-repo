# marchat

<img src="assets/marchat-transparent.svg" alt="marchat - terminal chat application" width="200" height="auto">

[![Go CI](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue?logo=go)](https://go.dev/dl/)
[![GitHub all releases](https://img.shields.io/github/downloads/Cod-e-Codes/marchat/total?logo=github)](https://github.com/Cod-e-Codes/marchat/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/codecodesxyz/marchat?logo=docker)](https://hub.docker.com/r/codecodesxyz/marchat)
[![Self-Host Weekly](https://img.shields.io/badge/Self--Host_Weekly-%23000080?Color=blue)](https://selfh.st/weekly/2025-07-25/)
[![mtkblogs.com](https://img.shields.io/badge/mtkblogs.com-%23FF6600?Color=white)](https://mtkblogs.com/2025/07/23/marchat-a-go-powered-terminal-chat-app-for-the-modern-user/)

A lightweight terminal chat with separate server and client binaries, real-time messaging over WebSockets, optional end-to-end encryption, and a flexible plugin ecosystem. Built for developers who prefer the command line and want reliable, self-hosted group chat with minimal operational overhead.

![Server Demo](assets/demo-server.gif "marchat server startup with ASCII art banner")
![Client Demo](assets/demo-client-1.gif "marchat client interface with chat and user list")

## Table of Contents  

- [Breaking Changes](#breaking-changes)
- [Overview](#overview)
- [Features](#features)  
- [Installation & Setup](#installation--setup)
  - [Binary Installation](#binary-installation)
  - [Docker Installation](#docker-installation)
  - [Source Installation](#source-installation)
  - [Database Migration for Development Version](#database-migration-for-development-version)
- [Quick Start](#quick-start)  
- [Configuration](#configuration)
  - [Version Compatibility](#version-compatibility)
- [TLS Support](#tls-support)
- [Usage](#usage)  
- [Security](#security)  
- [Troubleshooting](#troubleshooting)  
- [Roadmap](#roadmap)
- [Getting Help](#getting-help)
- [Contributing](#contributing)
- [Appreciation](#appreciation)

## Breaking Changes

> [!IMPORTANT]
> **Database Schema Migration Required** - Development Version Only
> 
> The current development version includes breaking changes that require database migration. These changes are not yet included in any release version.

### What's Changed

The development version introduces **per-user message state tracking** to fix the "frozen message history" bug where banned/unbanned users could only see messages from before their ban. This enhancement requires database schema changes that will affect existing installations.

### Who Is Affected

- **✅ NOT AFFECTED**: Users of published releases (v0.2.0-beta.5 and earlier)
- **⚠️ AFFECTED**: Users building from current source code
- **⚠️ AFFECTED**: Users running development builds

### Required Actions

**Before building from source:**
1. **Backup your database:**
   ```bash
   cp ./config/marchat.db ./config/marchat.db.backup
   ```

2. **Build and run** - migration happens automatically during server startup
3. **Verify migration** - check server logs for migration messages

### Migration Details

- **New table**: `user_message_state` for tracking per-user message history
- **Schema update**: `messages` table gets `message_id` column
- **Automatic migration**: Existing messages get `message_id = id`
- **Performance**: New indexes added for efficient queries
- **Duration**: Typically under 30 seconds for most installations

### Rollback Procedure

If migration fails or you need to downgrade:
```bash
# Stop the server
# Restore from backup
cp ./config/marchat.db.backup ./config/marchat.db
# Restart with previous version
```

### Benefits After Migration

- **Improved user experience**: Banned/unbanned users see complete message history
- **Better moderation**: Clean slate for users after ban/unban cycles
- **Enhanced performance**: Optimized queries with new indexes
- **Future-proof**: Foundation for advanced message tracking features

## Overview

marchat started as a fun weekend project for father-son coding sessions and has since evolved into a lightweight, self-hosted terminal chat application designed specifically for developers who love the command line. It currently runs with a local SQLite database and real-time messaging over WebSockets, with planned support for PostgreSQL and MySQL to enable greater scalability and flexibility.

**Key Benefits:**
- **Self-hosted**: No external services required
- **Cross-platform**: Runs on Linux, macOS, and Windows
- **Secure**: Optional E2E encryption with X25519/ChaCha20-Poly1305
- **Extensible**: Plugin ecosystem for custom functionality
- **Lightweight**: Minimal resource usage, perfect for servers

## Features

| Feature | Description |
|---------|-------------|
| **Terminal UI** | Beautiful TUI built with Bubble Tea |
| **Real-time Chat** | Fast WebSocket-based messaging with a lightweight SQLite backend |
| **Plugin System** | Install and manage plugins via `:store` and `:plugin` commands |
| **E2E Encryption** | Optional X25519 key exchange with ChaCha20-Poly1305 |
| **File Sharing** | Send files up to 1MB with `:sendfile` |
| **Admin Controls** | User management, bans, and database operations with improved ban/unban experience |
| **Themes** | Choose from patriot, retro, or modern themes |
| **Docker Support** | Containerized deployment with security features |
| **Enhanced User Experience** | Improved message history persistence after moderation actions |

| Cross-Platform File Sharing          | Theme Switching                         |
|------------------------------------|---------------------------------------|
| <img src="assets/mobile-file-transfer.jpg" width="300"/> | <img src="assets/theme-switching.jpg" width="300"/> |

*marchat running on Android via Termux, demonstrating file transfer through reverse proxy and real-time theme switching*

## Installation & Setup

### Binary Installation

**Download pre-built binaries for v0.2.0-beta.5:**

```bash
# Linux
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.2.0-beta.5/marchat-v0.2.0-beta.5-linux-amd64.zip
unzip marchat-v0.2.0-beta.5-linux-amd64.zip

# macOS
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.2.0-beta.5/marchat-v0.2.0-beta.5-darwin-amd64.zip
unzip marchat-v0.2.0-beta.5-darwin-amd64.zip

# Windows
# Download from GitHub releases page
```

### Docker Installation

**Pull from Docker Hub:**

```bash
# Latest release
docker pull codecodesxyz/marchat:v0.2.0-beta.5

# Run with environment variables
docker run -d \
  -p 8080:8080 \
  -e MARCHAT_ADMIN_KEY=$(openssl rand -hex 32) \
  -e MARCHAT_USERS=admin1,admin2 \
  codecodesxyz/marchat:v0.2.0-beta.5
```

**Using Docker Compose:**

```yaml
# docker-compose.yml
version: '3.8'
services:
  marchat:
    image: codecodesxyz/marchat:v0.2.0-beta.5
    ports:
      - "8080:8080"
    environment:
      - MARCHAT_ADMIN_KEY=${MARCHAT_ADMIN_KEY}
      - MARCHAT_USERS=${MARCHAT_USERS}
    volumes:
      - ./config:/marchat/config
```

### Docker/Unraid Deployment Notes

> [!NOTE]
> **SQLite Database Permissions**: SQLite requires write permissions on both the database file and its directory. Incorrect permissions may cause runtime errors on Docker/Unraid setups.
>
> **Automatic Fix**: The Docker image now automatically creates the complete directory structure (`/marchat/server/`) with all necessary subdirectories (config, db, data, plugins) and sets proper ownership at startup. This resolves permission issues that previously required manual intervention.
>
> **Manual Fix** (if needed): Create the required directories and ensure proper ownership:
> ```bash
> mkdir -p ./config/db
> chown -R 1000:1000 ./config/db
> chmod 775 ./config/db
> ```
> The container user (UID 1000) must match the ownership of these folders/files.

> [!WARNING]
> **Development Version Database Changes**: If building from source or using development builds, the database schema changes require proper volume mounting for persistence. Ensure your Docker setup includes volume mounts for the database directory to preserve data across container restarts.

### Source Installation

**Prerequisites:**
- Go 1.23+ ([download](https://go.dev/dl/))
- For Linux clipboard support: `sudo apt install xclip` (Ubuntu/Debian) or `sudo yum install xclip` (RHEL/CentOS)

**Build from source:**

> [!WARNING]
> **Development Version Database Changes**
> 
> Building from the current source includes database schema changes that require migration. Back up your database before building:
> ```bash
> cp ./config/marchat.db ./config/marchat.db.backup
> ```

```bash
git clone https://github.com/Cod-e-Codes/marchat.git
cd marchat
go mod tidy
go build -o marchat-server ./cmd/server
go build -o marchat-client ./client
chmod +x marchat-server marchat-client
```

### Database Migration for Development Version

When building from the current source code, the server automatically performs database schema migration during startup. This migration is required to support the new per-user message state tracking feature.

#### When Migration Occurs

- **First startup** after building from source with the new code
- **Automatic detection** of existing database schema
- **Safe migration** that preserves all existing message data

#### What Happens During Migration

1. **Schema Creation**: New `user_message_state` table is created
2. **Column Addition**: `message_id` column added to `messages` table
3. **Data Migration**: Existing messages get `message_id = id`
4. **Index Creation**: Performance indexes added for efficient queries
5. **Verification**: Server logs migration completion

#### Expected Server Output

```
2025/01/XX XX:XX:XX Warning: failed to migrate existing messages: <nil>
2025/01/XX XX:XX:XX marchat WebSocket server running on :8080
```

The warning message is normal and indicates successful migration.

#### Manual Verification

After migration, verify the new schema:
```bash
# Check if new table exists
sqlite3 ./config/marchat.db ".schema user_message_state"

# Verify message_id column
sqlite3 ./config/marchat.db "SELECT COUNT(*) FROM messages WHERE message_id > 0;"
```

#### Rollback if Migration Fails

If migration fails or you encounter issues:
```bash
# Stop the server
# Restore from backup
cp ./config/marchat.db.backup ./config/marchat.db
# Rebuild with previous version
git checkout v0.2.0-beta.5
go build -o marchat-server ./cmd/server
```

## Quick Start

### 1. (Recommended) Generate Secure Admin Key

For security, generate a strong random key to use as your admin key. This step is recommended but you can set any non-empty string as the admin key.

```bash
openssl rand -hex 32
```

### 2. Start Server

```bash
# Set environment variables
export MARCHAT_ADMIN_KEY="your-generated-key"
export MARCHAT_USERS="admin1,admin2"

# Start server
./marchat-server
```

### 3. Connect Client

```bash
# Connect as admin
./marchat-client --username admin1 --admin --admin-key your-generated-key --server ws://localhost:8080/ws

# Connect as regular user
./marchat-client --username user1 --server ws://localhost:8080/ws
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MARCHAT_ADMIN_KEY` | Yes | - | Admin authentication key |
| `MARCHAT_USERS` | Yes | - | Comma-separated admin usernames |
| `MARCHAT_PORT` | No | `8080` | Server port |
| `MARCHAT_DB_PATH` | No | `./config/marchat.db` | Database file path |
| `MARCHAT_LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `MARCHAT_CONFIG_DIR` | No | Auto-detected | Custom config directory |
| `MARCHAT_TLS_CERT_FILE` | No | - | Path to TLS certificate file |
| `MARCHAT_TLS_KEY_FILE` | No | - | Path to TLS private key file |

### Version Compatibility

#### Database Schema Versioning

marchat uses database schema versioning to ensure compatibility between different versions. The current development version introduces schema version 2, which includes per-user message state tracking.

#### Compatibility Matrix

| Version | Schema Version | Compatible With | Migration Required |
|---------|----------------|-----------------|-------------------|
| v0.2.0-beta.5 | 1 | v0.2.0-beta.5 and earlier | No |
| Development | 2 | Development builds only | Yes (automatic) |

#### Downgrade Implications

**⚠️ Important**: Downgrading from development version to v0.2.0-beta.5 will break the database schema. The development version's new tables and columns are not recognized by older versions.

**To downgrade safely:**
1. Restore from backup created before migration
2. Or create a fresh database with the older version

#### Multiple Environment Management

When maintaining multiple environments (development, staging, production):

- **Use separate databases** for each environment
- **Backup before schema changes** in each environment
- **Test migrations** in development before applying to production
- **Coordinate deployments** to avoid schema version mismatches

### Configuration File

Create `config.json` for client configuration:

```json
{
  "username": "your-username",
  "server_url": "ws://localhost:8080/ws",
  "theme": "patriot",
  "twenty_four_hour": true
}
```

## TLS Support

TLS (Transport Layer Security) enables secure WebSocket connections using `wss://` instead of `ws://`. This is essential for production deployments and when exposing the server over the internet.

### When to Use TLS

- **Public deployments**: When the server is accessible from the internet
- **Production environments**: For enhanced security and privacy
- **Corporate networks**: When required by security policies
- **HTTPS reverse proxies**: When behind nginx, traefik, or similar

### Enabling TLS

TLS is optional but recommended for secure deployments. To enable TLS:

1. **Obtain SSL/TLS certificates** (self-signed for testing, CA-signed for production)
2. **Set environment variables**:
   ```bash
   export MARCHAT_TLS_CERT_FILE="/path/to/cert.pem"
   export MARCHAT_TLS_KEY_FILE="/path/to/key.pem"
   ```
3. **Start the server** - it will automatically detect TLS configuration

### Example Configuration

**With TLS (recommended for production):**
```bash
# Generate self-signed certificate for testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Set environment variables
export MARCHAT_ADMIN_KEY="your-secure-key"
export MARCHAT_USERS="admin1,admin2"
export MARCHAT_TLS_CERT_FILE="./cert.pem"
export MARCHAT_TLS_KEY_FILE="./key.pem"

# Start server (will show wss:// in banner)
./marchat-server
```

**Without TLS (development/testing):**
```bash
# No TLS certificates set
export MARCHAT_ADMIN_KEY="your-secure-key"
export MARCHAT_USERS="admin1,admin2"

# Start server (will show ws:// in banner)
./marchat-server
```

### Client Connection

The client connection URL automatically reflects the server's TLS status:

- **TLS enabled**: Connect to `wss://host:port/ws`
- **TLS disabled**: Connect to `ws://host:port/ws`

The server banner displays the correct WebSocket URL scheme based on TLS configuration.

## Usage

### Basic Commands

| Command | Description | Example |
|---------|-------------|---------|
| `:theme <name>` | Switch theme | `:theme patriot` |
| `:time` | Toggle 12/24-hour format | `:time` |
| `:clear` | Clear chat buffer | `:clear` |
| `:sendfile <path>` | Send file (<1MB) | `:sendfile document.txt` |
| `:savefile <name>` | Save received file | `:savefile received.txt` |

### Plugin Commands

| Command | Description | Admin Only |
|---------|-------------|------------|
| `:store` | Open plugin store | No |
| `:plugin list` | List installed plugins | No |
| `:plugin install <name>` | Install plugin | No |
| `:plugin uninstall <name>` | Uninstall plugin | Yes |

### Admin Commands

| Command | Description | Example |
|---------|-------------|---------|
| `:cleardb` | Wipe server database | `:cleardb` |
| `:kick <username>` | Disconnect user | `:kick user1` |
| `:ban <username>` | Ban user for 24h with improved user experience after unban | `:ban user1` |
| `:unban <username>` | Remove user ban with clean message history restoration | `:unban user1` |

**Connect as admin:**
```bash
./marchat-client --username admin1 --admin --admin-key your-key --server ws://localhost:8080/ws
```

> [!NOTE]
> **Enhanced Ban/Unban Experience**: The development version automatically manages user message states during ban/unban operations, ensuring users see complete message history when they reconnect after being unbanned.

### E2E Encryption Commands

| Command | Description | Example |
|---------|-------------|---------|
| `:showkey` | Display public key | `:showkey` |
| `:addkey <user> <key>` | Add user's public key | `:addkey alice <base64-key>` |

**Enable E2E encryption:**
```bash
./marchat-client --e2e --keystore-passphrase your-passphrase --username alice
```

## Security

### Critical Security Warnings

> [!WARNING]
> Change default admin key immediately
>  The default admin key `changeme` is insecure. Generate a secure key:
```bash
openssl rand -hex 32
```

### Security Best Practices

1. **Generate Secure Keys:**
   ```bash
   # Admin key
   openssl rand -hex 32
   
   # JWT secret (optional)
   openssl rand -base64 32
   ```

2. **Secure File Permissions:**
   ```bash
   # Secure database file
   chmod 600 ./config/marchat.db
   
   # Secure config directory
   chmod 700 ./config
   ```

3. **Production Deployment:**
   - Use `wss://` for secure WebSocket connections
   - Implement reverse proxy (nginx/traefik)
   - Restrict server access to trusted networks
   - Use Docker secrets for sensitive environment variables

### E2E Encryption

When enabled, E2E encryption provides:
- **Forward Secrecy**: Unique session keys per conversation
- **Server Privacy**: Server cannot read encrypted messages
- **Key Management**: Local encrypted keystore with passphrase protection

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| **Connection failed** | Verify server URL uses `ws://` or `wss://` |
| **TLS certificate errors** | Ensure certificate and key files are readable and valid |
| **Admin commands not working** | Ensure `--admin` flag and correct `--admin-key` |
| **Clipboard not working (Linux)** | Install `xclip`: `sudo apt install xclip` |
| **Permission denied (Docker)** | Rebuild with correct UID/GID: `docker-compose build --build-arg USER_ID=$(id -u)` |
| **Port already in use** | Change port: `export MARCHAT_PORT=8081` |
| **Database migration fails** | Ensure proper database file permissions and backup before building from source |
| **Message history missing after update** | Expected behavior - user message states reset for improved ban/unban experience |
| **Server fails to start after source build** | Check database permissions and consider manual schema migration |

### Network Connectivity

**Local Network:**
```bash
# Ensure server binds to all interfaces
export MARCHAT_PORT=8080
./marchat-server
```

**Remote Access with Cloudflare Tunnel:**
```bash
# Install cloudflared
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared

# Create tunnel
./cloudflared tunnel --url http://localhost:8080
```

## Roadmap  

See the [project roadmap](ROADMAP.md) for planned features, performance enhancements, and future development goals.

## Getting Help

- **GitHub Issues**: [Report bugs](https://github.com/Cod-e-Codes/marchat/issues)
- **GitHub Discussions**: [Ask questions](https://github.com/Cod-e-Codes/marchat/discussions)
- **Documentation**: [Plugin Ecosystem](PLUGIN_ECOSYSTEM.md)
- **Security**: [Security Policy](SECURITY.md)

## Contributing

We welcome contributions! See the [contribution guidelines](CONTRIBUTING.md) for:
- Development setup
- Code style guidelines
- Pull request process

**Quick Start for Contributors:**
```bash
git clone https://github.com/Cod-e-Codes/marchat.git
cd marchat
go mod tidy
go test ./...
```

---

## Appreciation

Special thanks to these wonderful communities and bloggers for featuring and supporting **marchat**:

- [Self-Host Weekly](https://selfh.st/weekly/2025-07-25/) by Ethan Sholly  
- [mtkblogs.com](https://mtkblogs.com/2025/07/23/marchat-a-go-powered-terminal-chat-app-for-the-modern-user/) by Reggie

---

**License**: [MIT License](LICENSE)

**Commercial Support**: Contact [cod.e.codes.dev@gmail.com](mailto:cod.e.codes.dev@gmail.com)
