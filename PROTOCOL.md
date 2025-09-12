# marchat Protocol Specification

This document outlines the communication protocol used by `marchat`, a terminal-based chat application built with Go and Bubble Tea. It covers WebSocket interactions, message formats, and expected client behavior. The protocol is designed for simplicity, extensibility, and ease of implementation for alternative clients or integrations.

---

## WebSocket Connection

Clients connect to the server via WebSocket:

```
/ws
```

The server may run over either `ws://` (HTTP) or `wss://` (HTTPS/TLS) depending on configuration. The connection scheme is determined by whether TLS certificates are provided to the server.

After a successful WebSocket upgrade, the client must immediately send a handshake message.

---

## Handshake

The handshake message introduces the user to the server. It must be the first message sent after connection.

### Format

```json
{
  "username": "alice",
  "admin": true,
  "admin_key": "your-admin-key"
}
```

### Fields

- `username` (string): **Required.** Display name. Must be unique among currently connected users.
- `admin` (bool): Optional. Request admin access. Defaults to `false`.
- `admin_key` (string): Required only if `admin` is `true`. Must match the server-configured key.

If `admin` is requested:

- The username must match one in the admin allowlist (case-insensitive).
- The provided key must match the server’s configured key.

Invalid handshakes (missing username, duplicate names, or invalid admin credentials) result in immediate connection termination.

---

## Message Format

All messages exchanged after handshake use JSON.

### Chat Messages

```json
{
  "sender": "alice",
  "content": "hello world",
  "created_at": "2025-07-24T15:04:00Z",
  "type": "text"
}
```

#### Fields

- `sender` (string): Username of the sender.
- `content` (string): Message text. Empty if type is `file`.
- `created_at` (string): RFC3339 timestamp.
- `type` (string): Either `"text"` or `"file"`.
- `file` (object, optional): Present only when `type` is `"file"`.

#### File Object

```json
{
  "filename": "screenshot.png",
  "size": 23456,
  "data": "<base64-encoded>"
}
```

Maximum file size is configurable (default 1MB). Files exceeding this size are rejected.
Configure via environment variables on the server:

- `MARCHAT_MAX_FILE_BYTES`: exact byte limit (takes precedence)
- `MARCHAT_MAX_FILE_MB`: size in megabytes

If neither is set, the default is 1MB.

### Server Events

Messages initiated by the server to update client state.

#### User List

```json
{
  "type": "userlist",
  "data": {
    "users": ["alice", "bob"]
  }
}
```

---

## Server Behavior

- On successful handshake:
  - Sends up to 100 recent messages from history.
  - Sends current user list.
- On user connect/disconnect:
  - Broadcasts updated user list.
- On message send:
  - Broadcasts to all connected clients.
- Messages are saved to SQLite and capped at 1000 messages.

---

## Message Retention

- Messages are stored in SQLite.
- The most recent 1000 messages are retained.
- Older messages are deleted automatically.

---

## Authentication

Admin status is optional and granted only if:

- `admin` is set to `true` in the handshake.
- `admin_key` matches the server’s configured key.
- `username` is on the allowed admin list, if configured.

If `admin` is not requested, `admin_key` is not required.

---

## Configuration

Client configuration is typically provided via a config file:

```json
{
  "username": "YOUR_USERNAME",
  "server_url": "ws://localhost:9090/ws",
  "theme": "patriot",
  "twenty_four_hour": true
}
```

**Note**: The `server_url` should use `ws://` for HTTP connections or `wss://` for HTTPS/TLS connections, depending on the server's TLS configuration.

Sensitive values (like `admin_key`) are passed only during handshake and are not stored in config.

---

## Notes on Extensibility

The protocol is intentionally simple and JSON-based. Future capabilities may include:

- Typing indicators, reactions
- Message deletion/editing
- Bot integrations and automation
- Admin commands and moderation tools

Clients and tools should use the `type` field to determine message kind.

---

## Example Workflow

1. Client connects via WebSocket to `/ws`
2. Sends handshake:

```json
{
  "username": "carol",
  "admin": false
}
```

3. Server responds with history and user list
4. Client sends message:

```json
{
  "sender": "carol",
  "content": "hey there",
  "created_at": "2025-07-24T15:09:00Z",
  "type": "text"
}
```

5. Server broadcasts the message to all clients

---

This document is intended to help developers build compatible clients, bots, or tools for `marchat`, or understand how the protocol works.

For questions or suggestions, please open a GitHub Discussion or Issue.

