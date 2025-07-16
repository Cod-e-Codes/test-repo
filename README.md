# marchat 🧃

[![Go Version](https://img.shields.io/badge/go-1.18%2B-blue?logo=go)](https://go.dev/dl/)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-repo-blue?logo=github)](https://github.com/Cod-e-Codes/marchat)

**Requires Go 1.18+**

A modern, retro-inspired terminal chat app for father-son coding sessions. Built with Go, Bubble Tea, and SQLite (pure Go driver, no C compiler required). Fast, hackable, and ready for remote pair programming.

---

## Features

- **Terminal UI**: Beautiful, scrollable chat using [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Go HTTP Server**: Simple, robust, and cross-platform
- **SQLite (pure Go)**: No C compiler needed (uses `modernc.org/sqlite`)
- **Usernames & Timestamps**: See who said what, and when
- **Color Themes**: Slack, Discord, AIM, or classic
- **Emoji Support**: ASCII emoji auto-conversion
- **Configurable**: Set username, server URL, and theme via config or flags
- **Easy Quit**: Press `q` or `ctrl+c` to exit the chat

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

### 3. Run the server (port 9090)
```sh
go run cmd/server/main.go
```

### 4. (Optional) Create a config file
Create `config.json` in the project root:
```json
{
  "username": "Cody",
  "server_url": "http://localhost:9090",
  "theme": "slack"
}
```

### 5. Run the client
```sh
# With flags:
go run client/main.go --username Cody --theme slack --server http://localhost:9090

# Or with config file:
go run client/main.go --config config.json
```

---

## Usage
- **Send messages**: Type and press Enter
- **Quit**: Press `ctrl+c` or `Esc`
- **Themes**: `slack`, `discord`, `aim`, or leave blank for default
- **Emoji**: `:), :(, :D, <3, :P` auto-convert to Unicode
- **Scroll**: Use Up/Down arrows or your mouse to scroll chat
- **Switch theme**: Type `:theme <name>` and press Enter
- **Clear chat (client only)**: Type `:clear` and press Enter
- **Clear all messages (wipe DB)**: Type `:cleardb` and press Enter (removes all messages for everyone)
- **Banner**: Status and error messages appear above chat

---

## Project Structure
```
marchat/
├── client/           # TUI client (Bubble Tea)
│   ├── main.go
│   └── config/config.go
├── cmd/server/       # Server entrypoint
│   └── main.go
├── server/           # Server logic (DB, handlers)
│   ├── db.go
│   ├── handlers.go
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
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go SQLite)

---

## Next Steps
- [ ] Persistent config file
- [ ] Avatars and richer themes
- [ ] WebSocket support
- [ ] Deploy to cloud (Fly.io, AWS, etc.)

---

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

---

## License

This project is licensed under the [MIT License](LICENSE).
