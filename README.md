# marchat 🧃

[![Go Version](https://img.shields.io/badge/go-1.18%2B-blue?logo=go)](https://go.dev/dl/)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)

A modern, retro-inspired terminal chat app for father-son coding sessions. Built with Go, Bubble Tea, and SQLite (pure Go driver, no C compiler required). Fast, hackable, and ready for remote pair programming.

---

## Features

- **Terminal UI**: Beautiful, scrollable chat using [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Go WebSocket Server**: Real-time, robust, and cross-platform
- **SQLite (pure Go)**: No C compiler needed (uses `modernc.org/sqlite`)
- **Usernames & Timestamps**: See who said what, and when
- **Color Themes**: Slack, Discord, AIM, or classic
- **Emoji Support**: ASCII emoji auto-conversion
- **Configurable**: Set username, server URL, and theme via config or flags
- **User List**: Live-updating user list panel with a fixed width (constant), and up to 20 users shown
- **Message Cap**: Only the last 100 messages are kept in memory for performance
- **Mention Highlighting**: Regex-based mention detection for `@username` (full-message highlight)
- **Ping/Pong Heartbeat**: Robust WebSocket connection with ping/pong heartbeat
- **Easy Quit**: Press `ctrl+c` or `Esc` to exit the chat
- **Graceful Shutdown**: Clean exit with panic prevention
- **Polished UI**: User list width is consistent, and the '+N more' line is styled (italic/dimmed) for clarity
- **Admin Security**: Only the configured admin user can connect as `admin`
- **Separate Admin HTTP URL**: For admin commands like `:cleardb`, you must provide the HTTP(S) base URL via `--admin-url`
- **ASCII Art Banner**: Server displays a beautiful banner on startup with connection URLs and admin info

---

## Admin Security: Restricting the `admin` Username & Admin HTTP URL

- Only the user specified by the server's `--admin-username` flag (default: `Cody`) can connect as `username=admin`.
- The admin secret is hardcoded as `changeme` in the server (change this for production).
- **Security note**: Always change the default admin secret in production deployments.
- To connect as admin, use:

  ```sh
  go run client/main.go --username admin --server wss://your-url/ws?real_user=Cody --admin-url https://your-url
  ```
  (Replace `Cody` and the URLs with your actual admin username and deployment.)
- All privileged commands (like `:cleardb`) are only available to the admin user, and require the HTTP(S) base URL for admin commands (not the WebSocket URL).
- Any other user attempting to connect as `admin` will be rejected by the server.
- **Note:** The `:cleardb` command will POST to `https://your-url/clear` (not the WebSocket URL).

---

## Quick Start

### 1. Clone the repo
```sh
git clone https://github.com/Cod-e-Codes/marchat.git
cd marchat
```

### 2. Install Go dependencies
```sh
go mod tidy
```

### 3. Run the server (port 9090, WebSocket)
```sh
go run cmd/server/main.go
```

**Optional:** Customize admin settings (admin key is the secret used to authorize admin commands):
```sh
go run cmd/server/main.go --admin-username YourName --admin-key your-secret-key
```

### 4. (Optional) Create a config file
Create `config.json` in the project root:
```json
{
  "username": "Cody",
  "server_url": "ws://localhost:9090/ws",
  "theme": "slack",
  "twenty_four_hour": true
}
```

### 5. Run the client
```sh
# With flags:
go run client/main.go --username Cody --theme slack --server ws://localhost:9090/ws

# Or with config file (loaded from current working directory):
go run client/main.go --config config.json
```

---

## Usage
- **Send messages**: Type and press Enter
- **Quit**: Press `ctrl+c` or `Esc` to exit the chat
- **Themes**: `slack`, `discord`, `aim`, or leave blank for default
- **Emoji**: `:), :(, :D, <3, :P` auto-convert to Unicode
- **Scroll**: Use Up/Down arrows or your mouse to scroll chat
- **Switch theme**: Type `:theme <name>` and press Enter (persists in config)
- **Toggle timestamp format**: Type `:time` and press Enter (persists in config)
- **ASCII art banner**: Displays connection info on server startup; can be disabled via config or flag
- **Clear chat (client only)**: Type `:clear` and press Enter (clears local buffer, doesn't affect others)
- **Clear all messages (wipe DB)**: Type `:cleardb` and press Enter (admin only, removes all messages for everyone)
- **Banner**: Status and error messages appear above chat
- **Mentions**: Use `@username` to highlight a user (full-message highlight, not partial)
- **User List**: Up to 20 users shown in a fixed-width panel, with a styled `+N more` indicator if more are online

---

## Project Structure
```
marchat/
├── client/           # TUI client (Bubble Tea)
│   ├── main.go
│   └── config/config.go
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
├── go.mod
├── go.sum
└── README.md
```

---

## Tech Stack
- [Go](https://golang.org/) 1.18+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) (TUI)
- [Lipgloss](https://github.com/charmbracelet/lipgloss) (styling)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go SQLite, no C compiler required)
- [Gorilla WebSocket](https://github.com/gorilla/websocket) (real-time messaging)

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
- **Too many users in user list**
  - Only up to 20 users are shown, with a styled `+N more` indicator if more users are online.
- **Cross-platform**: Runs on Linux, macOS, and Windows terminals
- **Firewall/Port**: Ensure port 9090 is open for remote connections

---

## Next Steps

### ✅ Completed or In Progress

- [x] **Admin username restriction for privileged commands** *(Partially implemented — logic in place, currently being debugged)*
- [x] **User list with live updates** *(Fully implemented; fixed-width styling pending optional)*
- [x] **Regex-based mention highlighting (full-message)** *(Implemented — highlights entire message if `@username` found)*
- [x] **Graceful shutdown and panic prevention** *(Implemented — server handles disconnects and client exits cleanly)*
- [x] **ASCII art banner on server startup with connection info** *(Implemented)*

### 🛠️ Still To Do

- [ ] **Message cap and efficient memory use**
  Prevent unbounded message growth by capping stored messages or using ring buffers.

- [ ] **UI polish: styled '+N more' line, userListWidth constant**
  Visual improvements to make layout and overflow cleaner.

- [ ] **Separate admin HTTP URL for privileged commands**
  Structure in place but currently non-functional — requires further debugging/finalization.

---

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

---

## License

This project is licensed under the [MIT License](LICENSE).
