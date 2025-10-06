# marchat

<img src="assets/marchat-transparent.svg" alt="marchat - terminal chat application" width="200" height="auto">

[![Go CI](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue?logo=go)](https://go.dev/dl/)
[![GitHub all releases](https://img.shields.io/github/downloads/Cod-e-Codes/marchat/total?logo=github)](https://github.com/Cod-e-Codes/marchat/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/codecodesxyz/marchat?logo=docker)](https://hub.docker.com/r/codecodesxyz/marchat)
[![mtkblogs.com](https://img.shields.io/badge/mtkblogs.com-%23FF6600?Color=white)](https://mtkblogs.com/2025/07/23/marchat-a-go-powered-terminal-chat-app-for-the-modern-user/)

A lightweight terminal chat with real-time messaging over WebSockets, optional E2E encryption, and a flexible plugin ecosystem. Built for developers who prefer the command line.

![Server Demo](assets/demo-server.gif "marchat server startup")
![Client Demo](assets/demo-client-1.gif "marchat client interface")

## Features

- **Terminal UI** - Beautiful TUI built with Bubble Tea
- **Real-time Chat** - Fast WebSocket messaging with SQLite backend
- **Plugin System** - Remote registry with `:store` and `:plugin` commands
- **E2E Encryption** - X25519/ChaCha20-Poly1305 with global encryption
- **File Sharing** - Send files up to 1MB (configurable) with interactive picker
- **Admin Controls** - User management, bans, database operations
- **Bell Notifications** - Audio alerts with `:bell` and `:bell-mention`
- **Themes** - System (default), patriot, retro, modern
- **Docker Support** - Containerized deployment
- **Health Monitoring** - `/health` endpoints with system metrics
- **Structured Logging** - JSON logs with component separation

| Cross-Platform | Theme Switching |
|---------------|----------------|
| <img src="assets/mobile-file-transfer.jpg" width="300"/> | <img src="assets/theme-switching.jpg" width="300"/> |

## Quick Start

### 1. Generate Admin Key
```bash
openssl rand -hex 32
```

### 2. Start Server
```bash
export MARCHAT_ADMIN_KEY="your-generated-key"
export MARCHAT_USERS="admin1,admin2"
./marchat-server

# With admin panel
./marchat-server --admin-panel

# With web panel
./marchat-server --web-panel
```

### 3. Connect Client
```bash
# Admin connection
./marchat-client --username admin1 --admin --admin-key your-key --server ws://localhost:8080/ws

# Regular user
./marchat-client --username user1 --server ws://localhost:8080/ws

# Or use interactive mode
./marchat-client
```

## Installation

**Binary Installation:**
```bash
# Linux (amd64)
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.8.0-beta.2/marchat-v0.8.0-beta.2-linux-amd64.zip
unzip marchat-v0.8.0-beta.2-linux-amd64.zip && chmod +x marchat-*

# macOS (amd64)
wget https://github.com/Cod-e-Codes/marchat/releases/download/v0.8.0-beta.2/marchat-v0.8.0-beta.2-darwin-amd64.zip
unzip marchat-v0.8.0-beta.2-darwin-amd64.zip && chmod +x marchat-*

# Windows - PowerShell
iwr -useb https://raw.githubusercontent.com/Cod-e-Codes/marchat/main/install.ps1 | iex
```

**Docker:**
```bash
docker pull codecodesxyz/marchat:v0.8.0-beta.2
docker run -d -p 8080:8080 \
  -e MARCHAT_ADMIN_KEY=$(openssl rand -hex 32) \
  -e MARCHAT_USERS=admin1,admin2 \
  codecodesxyz/marchat:v0.8.0-beta.2
```

**From Source:**
```bash
git clone https://github.com/Cod-e-Codes/marchat.git && cd marchat
go mod tidy
go build -o marchat-server ./cmd/server
go build -o marchat-client ./client
```

## Configuration

### Essential Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MARCHAT_ADMIN_KEY` | Yes | - | Admin authentication key |
| `MARCHAT_USERS` | Yes | - | Comma-separated admin usernames |
| `MARCHAT_PORT` | No | `8080` | Server port |
| `MARCHAT_DB_PATH` | No | `./config/marchat.db` | Database file path |
| `MARCHAT_TLS_CERT_FILE` | No | - | TLS certificate (enables wss://) |
| `MARCHAT_TLS_KEY_FILE` | No | - | TLS private key |
| `MARCHAT_GLOBAL_E2E_KEY` | No | - | Base64 32-byte global encryption key |
| `MARCHAT_MAX_FILE_BYTES` | No | `1048576` | Max file size (1MB default) |

**Additional variables:** `MARCHAT_LOG_LEVEL`, `MARCHAT_CONFIG_DIR`, `MARCHAT_BAN_HISTORY_GAPS`, `MARCHAT_PLUGIN_REGISTRY_URL`, `MARCHAT_MAX_FILE_MB`

## Admin Commands

| Command | Description | Hotkey |
|---------|-------------|--------|
| `:cleardb` | Wipe server database | - |
| `:ban <user>` | Permanent ban | - |
| `:kick <user>` | 24h temporary ban | - |
| `:unban <user>` | Remove permanent ban | - |
| `:allow <user>` | Override kick early | `Ctrl+Shift+A` |
| `:cleanup` | Clean stale connections | - |
| `:forcedisconnect <user>` | Force disconnect user | - |

## User Commands

| Command | Description |
|---------|-------------|
| `:theme <name>` | Switch theme (system/patriot/retro/modern) |
| `:time` | Toggle 12/24-hour format |
| `:clear` | Clear chat buffer |
| `:sendfile [path]` | Send file (or open picker) |
| `:savefile <name>` | Save received file |
| `:code` | Open code composer |
| `:bell` | Toggle message notifications |
| `:bell-mention` | Toggle mention-only notifications |
| `:store` | Browse plugin store |
| `:plugin install/list/uninstall` | Manage plugins |

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+H` | Toggle help overlay |
| `Enter` | Send message |
| `Esc` | Quit |
| `↑/↓` | Scroll history |
| `PgUp/PgDn` | Page through history |
| `Ctrl+C/V/X` | Copy/Paste/Cut |
| `Ctrl+A` (server) | Open admin panel |

## Admin Panels

### Terminal Admin Panel
Press `Ctrl+A` when running `./marchat-server --admin-panel` to access:
- Real-time server statistics
- User management
- Plugin configuration
- Database operations

### Web Admin Panel
Access at `http://localhost:8080/admin` when running `./marchat-server --web-panel`:
- Secure session-based login
- Live dashboard with metrics
- RESTful API endpoints
- CSRF protection

## TLS Support

Enable secure WebSocket connections (wss://):

```bash
# Generate self-signed cert (testing)
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Configure TLS
export MARCHAT_TLS_CERT_FILE="./cert.pem"
export MARCHAT_TLS_KEY_FILE="./key.pem"
./marchat-server

# Connect with TLS
./marchat-client --server wss://localhost:8080/ws

# Skip verification (dev only)
./marchat-client --skip-tls-verify --server wss://localhost:8080/ws
```

## E2E Encryption

Global encryption for secure group chat:

```bash
# Generate global key
openssl rand -base64 32

# Share key with all clients
export MARCHAT_GLOBAL_E2E_KEY="your-generated-key"

# Connect with E2E
./marchat-client --e2e --keystore-passphrase your-pass --username alice --server ws://localhost:8080/ws
```

## Plugin System

```bash
# Browse available plugins
:store

# Install plugin
:plugin install echo

# List installed
:plugin list

# Custom registry
export MARCHAT_PLUGIN_REGISTRY_URL="https://my-registry.com/plugins.json"
```

## Security Best Practices

1. **Generate secure keys** with `openssl rand -hex 32`
2. **Use TLS** in production (`wss://`)
3. **Secure file permissions**: `chmod 600 marchat.db && chmod 700 config/`
4. **Implement reverse proxy** (nginx/traefik)
5. **Restrict network access** to trusted IPs

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Connection failed | Verify `ws://` or `wss://` protocol |
| Admin commands not working | Check `--admin` flag and `--admin-key` |
| Clipboard issues (Linux) | Install xclip: `sudo apt install xclip` |
| Port in use | Change port: `export MARCHAT_PORT=8081` |
| TLS errors | Use `--skip-tls-verify` for dev with self-signed certs |
| Username taken | Admin use `:forcedisconnect <user>` or wait 5min for auto-cleanup |
| Global E2E errors | Verify key with `openssl rand -base64 32` |

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Test scripts
./test.sh          # Linux/macOS
.\test.ps1         # Windows
```

**Overall coverage: 9.3%** - See [TESTING.md](TESTING.md) for details.

## Documentation

- [Plugin Ecosystem](PLUGIN_ECOSYSTEM.md)
- [Roadmap](ROADMAP.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Testing Guide](TESTING.md)

## Getting Help

- [Report bugs](https://github.com/Cod-e-Codes/marchat/issues)
- [Ask questions](https://github.com/Cod-e-Codes/marchat/discussions)
- Commercial support: [cod.e.codes.dev@gmail.com](mailto:cod.e.codes.dev@gmail.com)

## Appreciation

Thanks to [Self-Host Weekly](https://selfh.st/weekly/2025-07-25/), [mtkblogs.com](https://mtkblogs.com/2025/07/23/marchat-a-go-powered-terminal-chat-app-for-the-modern-user/), and [Terminal Trove](https://terminaltrove.com/) for featuring marchat!

See [CONTRIBUTORS.md](CONTRIBUTORS.md) for full contributor list.

---

**License**: [MIT License](LICENSE)
