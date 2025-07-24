# marchat 🧃

[![Go CI](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/Cod-e-Codes/marchat/actions/workflows/go.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)
[![Go Version](https://img.shields.io/badge/go-1.24%2B-blue?logo=go)](https://go.dev/dl/)
[![cloudflared](https://img.shields.io/badge/cloudflared-download-ff6f00?logo=cloudflare)](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/)

## Table of Contents

- [Beta Release](#beta-release)
- [Features](#features)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Project Structure](#project-structure)
- [Admin Mode](#admin-mode-privileged-commands--security)
- [Security](#security)
- [Tech Stack](#tech-stack)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## What is this?

`marchat` is a minimalist terminal-based group chat app designed for real-time, distraction-free conversations. Whether you're pair programming, self-hosting a LAN party, or just chatting from two terminals, it's lightweight, hackable, and built for fun.

## Why Marchat?

Built for father-son coding sessions, marchat is about sharing the joy of hacking, learning, and chatting in a terminal. It's a fun, retro-inspired project for anyone who loves the command line, real-time collaboration, or just wants a simple, self-hosted chat.

---

## Beta Release

`marchat` is currently in a pre-release phase with version `v0.1.0-beta.2`. This is the second public beta release, featuring prebuilt binaries for Linux, Windows, and macOS (amd64 only). The release includes both `marchat-server` and `marchat-client` executables, allowing you to test the application without building from source. This release includes clipboard support and bug fixes from `v0.1.0-beta.1`.

> [!IMPORTANT]
> This is a beta release intended for early testing and feedback. While stable for general use, some features may change or be refined before the first stable release. Please report any issues or suggestions on the [GitHub Issues page](https://github.com/Cod-e-Codes/marchat/issues).

### Installing the Beta Release

1. **Download the binaries**:
   - Visit the [v0.1.0-beta.2 release page](https://github.com/Cod-e-Codes/marchat/releases/tag/v0.1.0-beta.2).
   - Download the appropriate archive for your platform (Linux, Windows, or macOS, amd64 only).
   - Extract the archive to a directory of your choice.

2. **Run the server**:
   ```sh
   ./marchat-server
   ```
   - Optionally, start the server with admin privileges:
     ```sh
     ./marchat-server --admin YourName --admin-key your-admin-key
     ```

3. **Run the client**:
   ```sh
   # Linux/macOS
   ./marchat-client --username Cody --theme patriot --server ws://localhost:9090/ws

   # Windows
   marchat-client.exe --username Cody --theme patriot --server ws://localhost:9090/ws
   ```
   - Alternatively, use a `config.json` file (see [Quick Start](#quick-start) for details).

> [!NOTE]
> For the beta release, we recommend using the prebuilt binaries (`marchat-server` and `marchat-client`) instead of building from source. The binaries are standalone and include all dependencies.

> [!TIP]
> To provide feedback on the beta release, create an issue on the [GitHub Issues page](https://github.com/Cod-e-Codes/marchat/issues) with details about your experience, including your platform and any bugs encountered. Check the [Full Changelog](https://github.com/Cod-e-Codes/marchat/commits/v0.1.0-beta.2) for details on what’s included in this release.

---

## Features

- **Terminal UI (TUI):** Beautiful, scrollable chat using [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Real-time WebSocket Chat:** Fast, robust, and cross-platform server/client
- **Themes:** Choose from `patriot`, `retro`, or `modern` for a unique look
- **Small File Sharing (<1MB):** Instantly send and receive small files with `:sendfile <path>` and save them with `:savefile <filename>`
- **Emoji Support:** Auto-converts common ASCII emoji (e.g., `:)`, `:(`, `:D`, `<3`, `:P`) to Unicode
- **Live User List:** See who’s online in a fixed-width, styled panel (up to 20 users shown)
- **@Mention Highlighting:** Messages with `@username` highlight for all users in the chat
- **Clipboard Support:** Copy (`Ctrl+C`), paste (`Ctrl+V`), cut (`Ctrl+X`), and select all (`Ctrl+A`) in the textarea
- **Admin Mode:** Privileged commands (like `:cleardb`) for authenticated admins only
- **Message Cap:**
  - Only the last 100 messages are kept in memory for client performance
  - The server database automatically caps messages at 1000; oldest messages are deleted to make room for new ones
- **Configurable:** Set username, server URL, and theme via config file or flags
- **Graceful Shutdown:** Clean exit and robust connection handling (ping/pong heartbeat)
- **ASCII Art Banner:** Server displays a beautiful banner with connection info on startup

---

## Prerequisites

- Install [Go 1.24+](https://go.dev/dl/) if you haven’t already (only needed if building from source)
  - *(Check with `go version` in your terminal)*
- (Optional, for remote access) Download [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/) (`cloudflared.exe` on Windows)
- (Optional, for clipboard support on Linux) Install `xclip` or `xsel` for `github.com/atotto/clipboard` functionality

## Quick Start

> [!NOTE]
> You can configure marchat via flags or a `config.json`. Flags override config file values. For the beta release, use the prebuilt binaries as described in [Beta Release](#beta-release).

### 1. Clone the repo (if building from source)
```sh
git clone https://github.com/Cod-e-Codes/marchat.git
cd marchat
```

### 2. Install Go dependencies (if building from source)
```sh
go mod tidy
```

### 3. Build the project (if building from source)
```sh
go build ./...
```

### 4. Run the server (port 9090, WebSocket)
Using the prebuilt binary:
```sh
./marchat-server
```
Or, if building from source:
```sh
go run cmd/server/main.go
```

> [!TIP]
> Start the server with `--admin` to register an admin username, and use `--admin-key` to secure access:
```sh
./marchat-server --admin YourName --admin-key your-admin-key
```

### 5. (Optional) Create a config file
Create `config.json` in the project root:
```json
{
  "username": "Cody",
  "server_url": "ws://localhost:9090/ws",
  "theme": "patriot",
  "twenty_four_hour": true
}
```

> [!NOTE]
> If no `config.json` is found, the client uses default values. Specify a custom config path with `--config`.

### 6. Run the client
Using the prebuilt binary:
```sh
# Linux/macOS
./marchat-client --username Cody --theme patriot --server ws://localhost:9090/ws

# Windows
marchat-client.exe --username Cody --theme patriot --server ws://localhost:9090/ws
```
Or, if building from source:
```sh
go run client/main.go --username Cody --theme patriot --server ws://localhost:9090/ws
```
Or with a config file:
```sh
./marchat-client --config config.json
```

> [!IMPORTANT]
> Ensure the server URL uses `ws://` for local development or `wss://` for production (secure WebSocket).

---

## Remote Access (Optional)

If you want to make your marchat server accessible from outside your local network (e.g., to chat with friends remotely), use [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/).

### 1. Download cloudflared
- [Get cloudflared for your OS](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/)

### 2. Start a tunnel to your local server
```sh
cloudflared tunnel --url http://localhost:9090
```

> [!TIP]
> Cloudflare provides a public `https://` URL. Convert it to `wss://.../ws` for the client (e.g., `https://bold-forest-cat.trycloudflare.com` becomes `wss://bold-forest-cat.trycloudflare.com/ws`).

### 3. Update your client config
```sh
./marchat-client --username Cody --admin --admin-key your-admin-key --server wss://bold-forest-cat.trycloudflare.com/ws
```

> [!NOTE]
> Temporary tunnels don’t require a Cloudflare account. For persistent tunnels, see the [Cloudflare Tunnel docs](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/).

---

## Usage

Interact with marchat using the following commands and features:

- **Send messages**: Type and press `Enter`.
- **File sharing**:
  - Send files (<1MB): `:sendfile <path>`
  - Save received files: `:savefile <filename>`
    - If a file with the same name exists, a numeric suffix will be added (e.g., `file[1]`, `file[2]`, etc.). The actual saved filename will be shown in the UI.
- **Emoji support**: Auto-converts ASCII emoticons to Unicode (`:)`, `:(`, `:D`, `<3`, `:P`).
- **Mentions**: Use `@username` to highlight a user (full message highlighted).
- **Clipboard operations**:
  - Copy text: `Ctrl+C` (copies textarea content to clipboard)
  - Paste text: `Ctrl+V` (appends clipboard content to textarea)
  - Cut text: `Ctrl+X` (copies textarea content to clipboard and clears textarea)
  - Select all: `Ctrl+A` (copies all textarea content to clipboard)
- **Scroll**: Use Up/Down arrows or mouse to navigate chat history.
- **Commands**:
  - `:theme <name>`: Switch theme (`patriot`, `retro`, `modern`; persists in config).
  - `:time`: Toggle 12/24-hour timestamp format (persists in config).
  - `:clear`: Clear local chat buffer (client-side only, does not affect others).
  - `:cleardb`: Wipe entire server database (admin only).
- **User list**: Displays up to 20 online users, with a styled `+N more` indicator for additional users.
- **ASCII art banner**: Shows connection info on server startup (disable via config or flag).
- **Quit**: Press `Esc` to exit.
- **Clipboard shortcuts**: `[Ctrl+C]` Copy, `[Ctrl+V]` Paste, `[Ctrl+X]` Cut, `[Ctrl+A]` Select All (copies all to clipboard).

> [!CAUTION]
> The `:cleardb` command permanently deletes all messages in the server database. Use with caution, as this action cannot be undone.

> [!NOTE]
> File transfers are limited to 1MB to ensure performance. Larger files should be shared via other methods.

---

## Project Structure
```
marchat/
├── client/           # TUI client (Bubble Tea)
│   ├── main.go
│   └── config/
│       └── config.go
├── cmd/server/       # Server entrypoint
│   └── main.go
├── server/           # Server logic (DB, handlers, WebSocket)
│   ├── db.go
│   ├── handlers.go
│   ├── client.go
│   ├── hub.go
│   └── schema.sql
├── shared/           # Shared types
│   └── types.go
├── config.json       # Example or user config file (see Quick Start)
├── go.mod
├── go.sum
└── README.md
```

Modular architecture: client, server logic, and shared types are separated for clarity and maintainability.

---

## Admin Mode: Privileged Commands & Security

> [!IMPORTANT]
> Admin commands like `:cleardb` require the `--admin` flag and a matching `--admin-key`. Only users listed as admins on the server can authenticate.

**Admin Key Configuration (Security Update):**

- The `--admin-key` flag is deprecated and will be removed in a future release.
- Set your admin key in `config.json` as `"admin_key": "your-admin-key"`.
- Alternatively, set the `MARCHAT_ADMIN_KEY` environment variable.
- If neither is set, admin mode is disabled for security.

**Example config.json:**
```json
{
  "username": "Cody",
  "server_url": "ws://localhost:9090/ws",
  "theme": "patriot",
  "twenty_four_hour": true,
  "admin_key": "your-admin-key"
}
```

**To connect as admin:**
```sh
./marchat-client --username Cody --admin --server wss://localhost:9090/ws
# (admin_key will be read from config or env)
```

> [!WARNING]
> Do not use the default admin key (`changeme`) in production. Change it immediately to prevent unauthorized access.

- Admin usernames are case-insensitive.
- The admin key is sent only during the WebSocket handshake.
- All admin actions use WebSocket (no HTTP endpoints).

---

## Security

> [!WARNING]
> For production deployments:
> - Change the default admin key (`changeme`) to a secure value.
> - Use `wss://` for secure WebSocket connections.
> - Ensure firewall rules allow your chosen port (default: 9090).
> - Consider a reverse proxy (e.g., nginx) for added security.

---

## Tech Stack
- [Go](https://golang.org/) 1.24+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) (TUI)
- [Lipgloss](https://github.com/charmbracelet/lipgloss) (styling)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go SQLite, no C compiler required)
- [Gorilla WebSocket](https://github.com/gorilla/websocket) (real-time messaging)
- [atotto/clipboard](https://github.com/atotto/clipboard) (clipboard operations for copy, paste, cut, and select all)

**Platform Support**: Runs on Linux, macOS, and Windows terminals supporting ANSI escape sequences. Clipboard functionality on Linux requires `xclip` or `xsel`.

---

## Troubleshooting

- **Panic: `close of closed channel`**
  - Fixed: The client now guards against double-close of internal channels.
- **Client fails to connect with `http://` URL**
  - Use a WebSocket URL: `ws://localhost:9090/ws` or `wss://...` for remote.
- **Mentions not highlighted**
  - Use `@username` exactly (word boundary, not substring).
- **User list not updating**
  - Ensure server and client are both up to date and using compatible protocols.
- **Messages not showing or chat not updating**
  - Check your WebSocket connection and server logs for errors.
- **Old messages missing from chat history**
  - The server database only keeps the most recent 1000 messages.
- **Too many users in user list**
  - Only up to 20 users are shown, with a styled `+N more` indicator.
- **Clipboard operations not working**
  - Ensure `xclip` or `xsel` is installed on Linux for `github.com/atotto/clipboard`.
  - Verify the textarea is focused when using `Ctrl+C`, `Ctrl+V`, `Ctrl+X`, or `Ctrl+A`.
- **Firewall/Port**: Ensure port 9090 is open for remote connections.
- **Admin commands**
  - Ensure `--admin` and `--admin-key` match server settings.

> [!TIP]
> When reporting bugs, include your version or commit hash for faster resolution. For beta release issues, specify that you’re using `v0.1.0-beta.2`.

---

## Feedback

Have ideas for improving marchat? Found a bug or want to request a feature?

- Found a bug? [Open an issue](https://github.com/Cod-e-Codes/marchat/issues)
- Got an idea or suggestion? [Start a discussion](https://github.com/Cod-e-Codes/marchat/discussions)
- Trying out the beta? [Open an issue](https://github.com/Cod-e-Codes/marchat/issues) and include your OS and version

> [!IMPORTANT]
> Please keep feedback respectful, constructive, and on-topic. It helps improve the project for everyone.

---

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). For the beta release, we especially welcome feedback on usability, bugs, and feature requests via the [GitHub Issues page](https://github.com/Cod-e-Codes/marchat/issues).

---

## Automation

- **Dependency Updates:** marchat uses [Dependabot](https://github.com/dependabot) to automatically check for and propose updates to Go module dependencies.
- **Continuous Integration:** All pushes and pull requests are checked by [GitHub Actions](https://github.com/Cod-e-Codes/marchat/actions) for build, test, and linting. Please ensure your PR passes CI before requesting review.

---

## License

This project is licensed under the [MIT License](LICENSE).
