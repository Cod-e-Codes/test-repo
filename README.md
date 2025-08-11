# marchat

<img src="assets/marchat-transparent.svg" alt="marchat - terminal chat application" width="200" height="auto">

[![Go CI](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue?logo=go)](https://go.dev/dl/)
[![Docker Pulls](https://img.shields.io/docker/pulls/codecodesxyz/marchat?logo=docker)](https://hub.docker.com/r/codecodesxyz/marchat)
[![Self-Host Weekly](https://img.shields.io/badge/Self--Host_Weekly-%23000080?Color=blue)](https://selfh.st/weekly/2025-07-25/)
[![mtkblogs.com](https://img.shields.io/badge/mtkblogs.com-%23FF6600?Color=white)](https://mtkblogs.com/2025/07/23/marchat-a-go-powered-terminal-chat-app-for-the-modern-user/)

A lightweight terminal chat with separate server and client binaries, real-time messaging over WebSockets, optional end-to-end encryption, and a flexible plugin ecosystem. Built for developers who prefer the command line and want reliable, self-hosted group chat with minimal operational overhead.

![Server Demo](assets/demo-server.gif "marchat server startup with ASCII art banner")
![Client Demo](assets/demo-client-1.gif "marchat client interface with chat and user list")

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation & Setup](#installation--setup)
  - [Binary Installation](#binary-installation)
  - [Docker Installation](#docker-installation)
  - [Source Installation](#source-installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [TLS Support](#tls-support)
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

### Real-World Usage

**Cross-Platform File Sharing**

![File Transfer Demo](assets/mobile-file-transfer.jpg "File transfer from Android Termux to desktop via reverse proxy")

**Theme Customization**

![Theme Demo](assets/theme-switching.jpg "Dynamic theme switching between patriot and retro themes")

*marchat running on Android via Termux, demonstrating file transfer through reverse proxy and real-time theme switching*

## Features

| Feature | Description |
|---------|-------------|
| **Terminal UI** | Beautiful TUI built with Bubble Tea |
| **Real-time Chat** | Fast WebSocket-based messaging with a lightweight SQLite backend |
| **Plugin System** | Install and manage plugins via `:store` and `:plugin` commands |
| **E2E Encryption** | Optional X25519 key exchange with ChaCha20-Poly1305 |
| **File Sharing** | Send files up to 1MB with `:sendfile` |
| **Admin Controls** | User management, bans, and database operations |
| **Themes** | Choose from patriot, retro, or modern themes |
| **Docker Support** | Containerized deployment with security features |

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
> **SQLite Database Permissions**: Some users have reported out-of-memory errors on Docker/Unraid systems due to SQLite write permission issues. SQLite requires write permissions on both the database file and its containing directory.
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
| `:ban <username>` | Ban user for 24h | `:ban user1` |
| `:unban <username>` | Remove user ban | `:unban user1` |

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
