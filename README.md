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

- [Overview](#overview)
- [Features](#features)  
- [Database Schema](#database-schema)
- [Installation & Setup](#installation--setup)
  - [Binary Installation](#binary-installation)
  - [Docker Installation](#docker-installation)
  - [Source Installation](#source-installation)
- [Quick Start](#quick-start)  
- [Configuration](#configuration)
- [TLS Support](#tls-support)
- [Plugin System](#plugin-system)
- [Ban History Gaps](#ban-history-gaps)
- [Usage](#usage)  
- [Security](#security)  
- [Troubleshooting](#troubleshooting)  
- [Roadmap](#roadmap)
- [Getting Help](#getting-help)
- [Contributing](#contributing)
- [Appreciation](#appreciation)

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
| **Plugin System** | Install and manage plugins via remote registry with `:store` and `:plugin` commands |
| **E2E Encryption** | Optional X25519 key exchange with ChaCha20-Poly1305, fully integrated message flow |
| **File Sharing** | Send files up to 1MB with `:sendfile` |
| **Admin Controls** | User management, bans, and database operations with improved ban/unban experience |
| **Themes** | Choose from patriot, retro, or modern themes |
| **Docker Support** | Containerized deployment with security features |
| **Enhanced User Experience** | Improved message history persistence after moderation actions |

| Cross-Platform File Sharing          | Theme Switching                         |
|------------------------------------|---------------------------------------|
| <img src="assets/mobile-file-transfer.jpg" width="300"/> | <img src="assets/theme-switching.jpg" width="300"/> |

*marchat running on Android via Termux, demonstrating file transfer through reverse proxy and real-time theme switching*

## Database Schema

> [!NOTE]
> **Automatic Migration**: Starting with v0.3.0-beta.1, all database schema changes are applied automatically during server startup. No manual migration steps are required.

### Schema History

- **v0.3.0-beta.1**: Introduced `user_message_state` table and `message_id` column for per-user message tracking
- **v0.3.0-beta.3**: Added `ban_history` table for ban history gaps feature
- **v0.3.0-beta.4**: Enhanced plugin system with remote registry support and improved E2E encryption integration
- **v0.3.0-beta.5**: Complete E2E encryption overhaul and stabilization - fixed blank message issue, improved error handling, and enhanced debugging

### Current Schema

The database includes these key tables:
- **messages**: Core message storage with `message_id` for tracking
- **user_message_state**: Per-user message history state
- **ban_history**: Ban/unban event tracking for history gaps feature

## Installation & Setup

### Binary Installation

**Download pre-built binaries for v0.3.0-beta.5:**

```bash
# Linux (amd64)
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.3.0-beta.5/marchat-v0.3.0-beta.5-linux-amd64.zip
unzip marchat-v0.3.0-beta.5-linux-amd64.zip
chmod +x marchat-server marchat-client

# macOS (amd64)
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.3.0-beta.5/marchat-v0.3.0-beta.5-darwin-amd64.zip
unzip marchat-v0.3.0-beta.5-darwin-amd64.zip
chmod +x marchat-server marchat-client

# Windows
# Download from GitHub releases page, extract the ZIP,
# and run marchat-server.exe and marchat-client.exe from PowerShell or CMD.

# Android/Termux (arm64)
pkg install wget unzip
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.3.0-beta.5/marchat-v0.3.0-beta.5-android-arm64.zip
unzip marchat-v0.3.0-beta.5-android-arm64.zip
chmod +x marchat-server marchat-client

```

### Docker Installation

**Pull from Docker Hub:**

```bash
# Latest release
docker pull codecodesxyz/marchat:v0.3.0-beta.5

# Run with environment variables
docker run -d \
  -p 8080:8080 \
  -e MARCHAT_ADMIN_KEY=$(openssl rand -hex 32) \
  -e MARCHAT_USERS=admin1,admin2 \
  codecodesxyz/marchat:v0.3.0-beta.5
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

### Source Installation

**Prerequisites:**
- Go 1.23+ ([download](https://go.dev/dl/))
- For Linux clipboard support: `sudo apt install xclip` (Ubuntu/Debian) or `sudo yum install xclip` (RHEL/CentOS)

**Build from source:**

```bash
git clone https://github.com/Cod-e-Codes/marchat.git
cd marchat
go mod tidy
go build -o marchat-server ./cmd/server
go build -o marchat-client ./client
chmod +x marchat-server marchat-client
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
| `MARCHAT_BAN_HISTORY_GAPS` | No | `true` | Enable ban history gaps (prevents banned users from seeing messages during ban periods) |
| `MARCHAT_PLUGIN_REGISTRY_URL` | No | GitHub registry | URL for plugin registry (default: https://raw.githubusercontent.com/Cod-e-Codes/marchat-plugins/main/registry.json) |

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

### TLS Verification Bypass

For development and testing with self-signed certificates, you can bypass TLS verification:

```bash
# Connect with TLS verification bypass
./marchat-client --skip-tls-verify --server wss://localhost:8080/ws

# Regular connection (with verification)
./marchat-client --server wss://localhost:8080/ws
```

> [!WARNING]
> **Security Warning**: Use `--skip-tls-verify` only for development and testing. Production deployments should use valid TLS certificates.

## Plugin System

The plugin system allows you to extend marchat's functionality with custom commands and features. Plugins are automatically downloaded from the configured registry.

### Plugin Registry

By default, marchat uses the GitHub plugin registry. You can configure a custom registry:

```bash
# Use default GitHub registry
export MARCHAT_PLUGIN_REGISTRY_URL="https://raw.githubusercontent.com/Cod-e-Codes/marchat-plugins/main/registry.json"

# Use custom registry
export MARCHAT_PLUGIN_REGISTRY_URL="https://my-registry.com/plugins.json"
```

### Plugin Commands

| Command | Description | Example |
|---------|-------------|---------|
| `:store` | Browse available plugins | `:store` |
| `:plugin install <name>` | Install a plugin | `:plugin install echo` |
| `:plugin uninstall <name>` | Remove a plugin | `:plugin uninstall echo` |
| `:plugin list` | List installed plugins | `:plugin list` |

### Available Plugins

- **Echo**: Simple echo plugin for testing

## Ban History Gaps

The ban history gaps feature prevents banned users from seeing messages that were sent during their ban periods. This creates a more effective moderation experience by ensuring users cannot access conversation history from when they were excluded from the chat.

### How It Works

When enabled, the system:
1. **Tracks ban events** in a dedicated `ban_history` table
2. **Records ban/unban timestamps** with admin attribution
3. **Filters message history** for users with ban records
4. **Maintains performance** by only filtering for users who have been banned

### Enabling Ban History Gaps

Set the environment variable to enable this feature:

```bash
# Enable ban history gaps (enabled by default)
export MARCHAT_BAN_HISTORY_GAPS=true

# Start server with feature enabled
./marchat-server
```

### Behavior Examples

**With Ban History Gaps Enabled:**
- User gets banned → cannot see new messages
- User gets unbanned → reconnects and sees only messages sent after their unban
- Messages sent during ban period are permanently hidden from that user

**With Ban History Gaps Disabled:**
- User gets banned → cannot see new messages
- User gets unbanned → reconnects and sees all messages (including those sent during ban)
- Standard behavior maintained for backward compatibility

### Performance Considerations

- **Minimal impact** on users who have never been banned
- **Database queries** only run for users with ban history
- **Automatic cleanup** of expired ban records
- **Indexed queries** for efficient ban period lookups

### Database Schema

The feature adds a new `ban_history` table:
```sql
CREATE TABLE ban_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    unbanned_at DATETIME,
    banned_by TEXT NOT NULL
);
```

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

### E2E Encryption Commands

| Command | Description | Example |
|---------|-------------|---------|
| `:showkey` | Display public key | `:showkey` |
| `:addkey <user> <key>` | Add user's public key | `:addkey alice <base64-key>` |

**Enable E2E encryption:**
```bash
./marchat-client --e2e --keystore-passphrase your-passphrase --username alice --server ws://localhost:8080/ws
```

**E2E encryption is now fully integrated and stabilized (v0.3.0-beta.5):**
- **Automatic encryption**: All outgoing messages are encrypted when `--e2e` is enabled
- **Automatic decryption**: Incoming encrypted messages are automatically decrypted
- **Session keys**: Each conversation uses unique session keys for isolation
- **Graceful fallback**: Failed decryption attempts show clear error messages instead of blank messages
- **Enhanced debugging**: Comprehensive logging for troubleshooting encryption issues
- **Mixed mode support**: E2E and non-E2E clients can coexist seamlessly
- **Improved error handling**: Clear error messages for missing keys, encryption failures, and keystore issues

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

#### v0.3.0-beta.5 Improvements

The latest release includes major E2E encryption improvements:
- **Fixed blank message issue**: Outgoing encrypted messages now display properly
- **Enhanced error handling**: Clear error messages for encryption failures
- **Improved debugging**: Comprehensive logging for troubleshooting
- **Better key management**: Improved session key derivation and storage
- **Mixed mode support**: Seamless coexistence of E2E and non-E2E clients
- **Graceful fallbacks**: Messages still send even if encryption fails

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
| **Server fails to start after source build** | Check database permissions - migrations are automatic |
| **Ban history gaps not working** | Ensure `MARCHAT_BAN_HISTORY_GAPS=true` is set (default) and database has `ban_history` table |
| **TLS certificate errors** | Use `--skip-tls-verify` flag for development with self-signed certificates |
| **Plugin installation fails** | Check `MARCHAT_PLUGIN_REGISTRY_URL` is accessible and registry format is valid |
| **E2E encryption not working** | Ensure `--e2e` flag is used and keystore passphrase is provided. Check debug logs for detailed error messages |
| **Blank encrypted messages** | Fixed in v0.3.0-beta.5 - ensure you're using the latest version and have added recipient public keys with `:addkey` |

### Network Connectivity

**Local Network:**
```bash
# Ensure server binds to all interfaces
export MARCHAT_PORT=8080
./marchat-server
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
