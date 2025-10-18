# Marchat Architecture Documentation

This document provides a comprehensive overview of the marchat system architecture, including component relationships, data flow, and design patterns.

## System Overview

Marchat is a self-hosted, terminal-based chat application built in Go with a client-server architecture. The system emphasizes security through end-to-end encryption, extensibility through a plugin system, and usability through multiple interface options.

## Core Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client TUI    │◄──►│  WebSocket      │◄──►│  Server Hub     │
│                 │    │  Communication  │    │                 │
│ • Chat Interface│    │ • JSON Messages │    │ • Message Hub   │
│ • File Transfer │    │ • E2E Encryption│    │ • User Mgmt     │
│ • Admin Panel   │    │ • Real-time     │    │ • Plugin Host   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Configuration   │    │ Shared Types    │    │ Database Layer  │
│ • Profiles      │    │ • Message Types │    │ • SQLite        │
│ • Encryption    │    │ • Crypto Utils  │    │ • Persistence   │
│ • Themes        │    │ • Protocols     │    │ • State Mgmt    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Component Architecture

### Client Application (`client/main.go`)

The client is a standalone terminal user interface built with the Bubble Tea framework. It's a complete application that can be built and run independently.

#### Core Models

- **`model`**: Main application state manager handling WebSocket communication, message rendering, and user interactions
- **`ConfigUIModel`**: Interactive configuration interface for server connection settings
- **`ProfileSelectionModel`**: Multi-profile management system for different server configurations
- **`SensitiveDataModel`**: Secure credential input for admin keys and encryption passphrases
- **`codeSnippetModel`**: Code block rendering with syntax highlighting and selection capabilities
- **`filePickerModel`**: Interactive file selection interface with filtering and preview
- **`NotificationManager`**: Notification system supporting bell sounds and desktop notifications

#### Key Features

- Real-time chat with message history and user list
- File sharing with configurable size limits (default 1MB)
- Theme system supporting built-in and custom themes
- Administrative commands for user management
- End-to-end encryption with global key support
- Code snippet rendering with syntax highlighting
- Clipboard integration for text operations
- URL detection and external opening

### Server Application (`cmd/server/main.go`)

The server is a standalone HTTP/WebSocket server application that provides real-time communication with plugin support and administrative interfaces.

#### Core Structures

- **`Hub`**: Central message routing system managing client connections, message broadcasting, and user state
- **`Client`**: Individual WebSocket connection handler with read/write pumps and command processing
- **`AdminPanel`**: Terminal-based administrative interface for server management
- **`WebAdminServer`**: Web-based administrative interface with session authentication
- **`HealthChecker`**: System health monitoring with metrics collection
- **`PluginCommandHandler`**: Plugin command routing and execution system

#### Key Features

- Real-time message broadcasting to connected clients
- User management including ban, kick, and allow operations
- Plugin command execution and management
- Database backup and maintenance operations
- System metrics collection and health monitoring
- Web-based admin panel with CSRF protection
- Health check endpoints for monitoring systems

### Server Library (`server/`)

The server package contains the core server logic and components that are used by the server application.

#### Core Components

- **WebSocket Handlers**: Connection management and message routing
- **Database Layer**: SQLite integration with message persistence
- **Admin Interfaces**: Both TUI and web-based administrative panels
- **Plugin Integration**: Plugin command handling and execution
- **Health Monitoring**: System metrics and health check endpoints

### Shared Components (`shared/`)

Common types and utilities used across client and server components.

#### Core Types

- **`Message`**: Standard chat message structure with encryption support
- **`EncryptedMessage`**: End-to-end encrypted message format
- **`Handshake`**: WebSocket connection authentication structure
- **`KeyPair`**: X25519 cryptographic identity for key exchange
- **`SessionKey`**: Derived encryption keys for message security
- **`FileMeta`**: File transfer metadata including name, size, and data

#### Encryption System

The encryption system provides end-to-end security using modern cryptographic primitives:

- **X25519**: Elliptic curve key exchange for secure key establishment
- **ChaCha20-Poly1305**: Authenticated encryption for message confidentiality and integrity
- **Global E2E**: Server-wide encryption key for simplified key management
- **Session Keys**: Derived keys for efficient message encryption

### Plugin System (`plugin/`)

Extensible architecture allowing custom functionality through external plugins.

#### Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Plugin SDK    │    │  Plugin Host    │    │ Plugin Manager  │
│                 │    │                 │    │                 │
│ • Core Interface│◄──►│ • Subprocess    │◄──►│ • Installation  │
│ • Communication │    │ • Lifecycle     │    │ • Store         │
│ • Base Classes  │    │ • JSON Protocol │    │ • Commands      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Plugin Store    │    │ License System  │    │ Command Handler │
│                 │    │                 │    │                 │
│ • TUI Interface │    │ • Validation    │    │ • Chat Commands │
│ • Registry      │    │ • Generation    │    │ • Integration   │
│ • Installation  │    │ • Caching       │    │ • Routing       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

#### Components

- **SDK** (`plugin/sdk/`): Core plugin interface definitions and base implementations
- **Host** (`plugin/host/`): Subprocess management and JSON-based communication
- **Manager** (`plugin/manager/`): Plugin installation, store integration, and command execution
- **Store** (`plugin/store/`): Terminal-based plugin browsing and installation interface
- **License** (`plugin/license/`): Cryptographic license validation for official plugins

#### Communication Protocol

Plugins communicate with the host through JSON messages over stdin/stdout:

- **Request Format**: `{"type": "message|command|init|shutdown", "command": "name", "data": {}}`
- **Response Format**: `{"type": "message|log", "success": true, "data": {}, "error": "message"}`
- **Message Types**: Initialization, message processing, command execution, graceful shutdown

### Configuration System (`config/`)

Flexible configuration management supporting multiple sources and interactive setup.

#### Configuration Sources

1. **Environment Variables**: Primary configuration method for production deployments
2. **`.env` Files**: Local development and testing configuration
3. **Interactive TUI**: User-friendly setup for initial configuration
4. **Profile System**: Multiple server configurations for different environments

#### Key Settings

- **Server Configuration**: Port, TLS certificates, admin authentication
- **Database Settings**: SQLite file path and connection parameters
- **Plugin Configuration**: Registry URL and installation directories
- **Security Settings**: Admin keys, encryption keys, and authentication
- **File Transfer**: Size limits and allowed file types
- **Logging**: Log levels and output destinations

### Command Line Tools (`cmd/`)

Additional command-line utilities for system management and plugin licensing.

#### Available Commands

- **`cmd/server/main.go`**: Main server application with interactive configuration
- **`cmd/license/main.go`**: Plugin license management and validation tool

#### License Tool Features

- **License Generation**: Create signed licenses for official plugins
- **License Validation**: Verify plugin license authenticity
- **Key Management**: Generate Ed25519 key pairs for signing
- **Cache Management**: Offline license validation support

## Data Flow and Communication

### Message Flow

```
Client: Input → Encrypt → WebSocket Send
Server: WebSocket Receive → Hub → Plugin Processing → Database
Server: Database → Hub → WebSocket Broadcast  
Client: WebSocket Receive → Decrypt → Display
```

### WebSocket Protocol

The WebSocket communication uses JSON messages with the following structure:

- **Handshake**: Initial authentication with username and admin credentials
- **Messages**: Chat messages with optional encryption
- **Commands**: Administrative and plugin commands
- **System Messages**: Connection status and user list updates

### Encryption Flow

1. **Key Generation**: Client generates X25519 keypair during initialization
2. **Key Exchange**: Global session key established with server
3. **Message Encryption**: Messages encrypted using ChaCha20-Poly1305
4. **Transport**: Encrypted data base64-encoded for JSON transport
5. **Storage**: Server stores encrypted messages in database

## Database Schema

### Tables

#### `messages`
```sql
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER DEFAULT 0,
    sender TEXT,
    content TEXT,
    created_at DATETIME,
    is_encrypted BOOLEAN DEFAULT 0,
    encrypted_data BLOB,
    nonce BLOB,
    recipient TEXT
);
```

#### `user_message_state`
```sql
CREATE TABLE user_message_state (
    username TEXT PRIMARY KEY,
    last_message_id INTEGER NOT NULL DEFAULT 0,
    last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

#### `ban_history`
```sql
CREATE TABLE ban_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    unbanned_at DATETIME,
    banned_by TEXT NOT NULL
);
```

### Key Features

- **Message ID Tracking**: Sequential message IDs for user state management
- **Encryption Support**: Binary storage for encrypted message data
- **Performance Indexes**: Optimized queries for message retrieval and user state
- **Message Cap**: Automatic cleanup maintaining 1000 most recent messages
- **Ban History**: Comprehensive tracking of user moderation actions

## Administrative Interfaces

### Terminal Admin Panel

The TUI-based admin panel provides real-time server management:

- **System Monitoring**: Live metrics including connections, memory usage, and message rates
- **User Management**: Ban, kick, and allow operations with history tracking
- **Plugin Management**: Install, enable, disable, and uninstall plugins
- **Database Operations**: Backup, restore, and maintenance functions
- **System Controls**: Force garbage collection and metrics reset

### Web Admin Panel

The web-based interface provides the same functionality through a browser:

- **Session Authentication**: Secure login with admin key validation
- **CSRF Protection**: Cross-site request forgery prevention
- **Real-time Updates**: Live data refresh without page reloads
- **RESTful API**: Programmatic access to administrative functions
- **Responsive Design**: Works across desktop and mobile devices

## Security Architecture

### Authentication

- **Admin Key**: Shared secret for administrative access
- **Session Management**: Secure session tokens with expiration
- **CSRF Protection**: Token-based request validation
- **Rate Limiting**: Protection against brute force attacks

### Encryption

- **End-to-End**: Client-side encryption with server-side key management
- **Key Derivation**: HKDF-based key derivation from passphrases
- **Forward Secrecy**: Session-based keys for message security
- **Authenticated Encryption**: ChaCha20-Poly1305 for confidentiality and integrity

### Input Validation

- **Plugin Names**: Regex-based validation preventing path traversal
- **Username Validation**: Case-insensitive matching with length limits
- **File Upload**: Type and size validation with safe path handling
- **Command Parsing**: Structured command validation and sanitization

## Performance Considerations

### Concurrency

- **Goroutine-based**: Concurrent handling of WebSocket connections
- **Channel Communication**: Non-blocking message passing between components
- **Connection Pooling**: Efficient database connection management
- **Plugin Isolation**: Separate processes prevent plugin crashes from affecting server

### Memory Management

- **Message Limits**: Automatic cleanup of old messages
- **Connection Tracking**: Efficient client state management
- **Plugin Lifecycle**: Proper cleanup of plugin subprocesses
- **Garbage Collection**: Configurable GC with monitoring

### Database Optimization

- **Indexed Queries**: Performance indexes on frequently queried columns
- **Batch Operations**: Efficient bulk message operations
- **Connection Reuse**: Persistent database connections
- **Query Optimization**: Prepared statements for common operations

## Development Patterns

### Code Organization

- **Package-based**: Clear separation of concerns across packages
- **Interface-driven**: Dependency injection through interfaces
- **Model-View-Update**: Bubble Tea pattern for TUI components
- **Command Pattern**: Structured handling of administrative actions

### Error Handling

- **Graceful Degradation**: System continues operating despite component failures
- **Plugin Isolation**: Plugin failures don't affect core functionality
- **Comprehensive Logging**: Structured logging with component identification
- **User Feedback**: Clear error messages and status indicators

### Testing Strategy

- **Unit Tests**: Component-level testing with mock dependencies
- **Integration Tests**: End-to-end testing of client-server communication
- **Plugin Testing**: Isolated testing of plugin functionality
- **Performance Testing**: Load testing for concurrent connections

## Build and Deployment

### Application Structure

marchat produces two main executables:

- **`marchat-client`**: Built from `client/main.go` - the terminal chat client
- **`marchat-server`**: Built from `cmd/server/main.go` - the WebSocket server

### Build System

- **Cross-Platform Builds**: Automated builds for Linux, macOS, Windows, and Android
- **Architecture Support**: AMD64, ARM64, and ARM32 variants
- **Release Scripts**: PowerShell and shell scripts for automated releases
- **Docker Support**: Containerized deployment with health checks

### Configuration Management

- **Environment Variables**: Primary configuration method for containers
- **Interactive Setup**: User-friendly initial configuration through TUI
- **Profile Management**: Multiple server configurations for different environments
- **Backward Compatibility**: Support for deprecated command-line flags

### Container Support

- **Docker**: Official Docker images for easy deployment
- **Environment-based**: Configuration through environment variables
- **Volume Mounting**: Persistent data and configuration storage
- **Health Checks**: Built-in health monitoring for orchestration

### Cross-Platform Support

- **Operating Systems**: Linux, macOS, Windows, and Android/Termux
- **Architecture Support**: AMD64, ARM64, and ARM32
- **Terminal Compatibility**: Works with most terminal emulators
- **File System**: Handles different path separators and permissions

## Extension Points

### Plugin Development

- **SDK**: Comprehensive plugin development kit
- **Documentation**: Detailed plugin development guides
- **Examples**: Reference implementations for common patterns
- **Registry**: Centralized plugin distribution and discovery

### Custom Themes

- **JSON Configuration**: Declarative theme definition
- **Color Schemes**: Comprehensive color palette support
- **Component Styling**: Granular control over UI elements
- **Dynamic Loading**: Runtime theme switching

### Alternative Frontends

- **WebSocket Protocol**: Standardized communication layer enabling any frontend
- **JSON IPC**: Structured messages for easy parsing and integration
- **Encryption Support**: X25519/ChaCha20-Poly1305 ensures secure messaging
- **Frontend Flexibility**: Architecture supports multiple frontend technologies
  - Web, desktop, or mobile clients can implement real-time chat, file transfer, and admin commands
- **Protocol Independence**: Frontends are decoupled from server implementation

### Administrative Extensions

- **Custom Commands**: Plugin-based command extensions
- **Webhooks**: External system integration
- **Metrics Export**: Custom monitoring and alerting
- **Backup Systems**: Custom backup and restore procedures
