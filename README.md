# marchat 🧃

> A father-son terminal chat app built with Go, Bubble Tea, and SQLite.

## 🧱 Stack

- Bubble Tea TUI client
- Go HTTP server
- SQLite message store

## 🚀 Getting Started

### Clone the repo

```bash
git clone <your-repo-url>
cd marchat
```

### Set up Go

```bash
go mod init marchat
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles/textinput
go get github.com/mattn/go-sqlite3
```

### Run server

```bash
go run server/main.go
```

### Create a config file (optional)

Create `config.json` in the root or pass `--config path` to the client:

```json
{
  "username": "Cody",
  "server_url": "http://localhost:8080",
  "theme": "slack"
}
```

### Run client

```bash
go run client/main.go --username Cody --theme slack
```

Or use the config file:

```bash
go run client/main.go --config config.json
```

### Themes
- `slack` (default)
- `discord`
- `aim`

### Emoji Support
- `:), :(, :D, <3, :P` are replaced with Unicode emojis in the chat log.

---

## ✅ Next Steps

* [ ] Add persistent config file
* [ ] Theme colors and avatars
* [ ] WebSocket mode (optional)
* [ ] Auto-refresh/polling optimizations
* [ ] Deploy to AWS EC2 or Fly.io

---

Enjoy chatting! 💬
