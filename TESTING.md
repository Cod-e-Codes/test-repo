# Marchat Test Suite

This document describes the test suite for the Marchat chat application.

## Overview

The Marchat test suite provides foundational coverage of the application's core functionality, including:

- **Unit Tests**: Testing individual components and functions in isolation
- **Integration Tests**: Testing the interaction between different components
- **Crypto Tests**: Testing cryptographic functions and E2E encryption
- **Database Tests**: Testing database operations and schema management
- **Server Tests**: Testing WebSocket handling, message routing, and user management

**Note**: This is a foundational test suite with good coverage for smaller utility packages but limited coverage for the main application components. Overall coverage is 9.3% across all packages.

## Test Structure

### Test Files

| File | Description | Coverage |
|------|-------------|----------|
| `config/config_test.go` | Configuration loading and validation | Environment variables, validation rules |
| `shared/crypto_test.go` | Cryptographic operations | Key generation, encryption, decryption, session keys |
| `shared/types_test.go` | Data structures and serialization | Message types, JSON marshaling/unmarshaling |
| `cmd/server/main_test.go` | Server main function and startup | Flag parsing, configuration validation, TLS setup, admin management |
| `server/handlers_test.go` | Server-side request handling | Database operations, message insertion, IP extraction |
| `server/hub_test.go` | WebSocket hub management | User bans, kicks, connection management |
| `server/integration_test.go` | End-to-end workflows | Message flow, ban flow, concurrent operations |
| `plugin/sdk/plugin_test.go` | Plugin SDK | Message types, JSON serialization, validation |
| `plugin/host/host_test.go` | Plugin Host | Plugin lifecycle, communication, enable/disable |
| `plugin/store/store_test.go` | Plugin Store | Registry management, platform resolution, filtering |
| `plugin/manager/manager_test.go` | Plugin Manager | Installation, uninstallation, command execution |
| `plugin/integration_test.go` | Plugin Integration | End-to-end plugin system workflows |

### Test Categories

#### 1. Unit Tests
- **Crypto Functions**: Key generation, encryption/decryption, session key derivation
- **Data Types**: Message structures, JSON serialization, validation
- **Utility Functions**: IP extraction, message sorting, database stats
- **Configuration**: Environment variable parsing, validation rules
- **Server Main**: Flag parsing, multi-flag handling, banner display, admin username normalization

#### 2. Integration Tests
- **Message Flow**: Complete message lifecycle from insertion to retrieval
- **User Management**: Ban/kick/unban workflows with database persistence
- **Concurrent Operations**: Thread-safe operations and race condition testing
- **Database Operations**: Schema creation, message capping, backup functionality

#### 3. Server Tests
- **WebSocket Handling**: Connection management, authentication, message routing
- **Hub Management**: User registration, broadcasting, cleanup operations
- **Admin Functions**: Ban management, user administration, system commands
- **Server Main Function**: Flag parsing, configuration validation, TLS setup, admin username normalization, banner display

## Running Tests

### Prerequisites

- Go 1.21 or later
- SQLite support (built into Go)
- PowerShell (for Windows test script)

### Basic Test Execution

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./server
go test ./shared
go test ./plugin/sdk
```

### Using Test Scripts

#### Linux/macOS
```bash
# Run the test suite
./test.sh
```

#### Windows PowerShell
```powershell
# Run basic test suite
.\test.ps1

# Run with coverage report
.\test.ps1 -Coverage

# Run with verbose output
.\test.ps1 -Verbose
```

### Test Coverage

Generate and view test coverage:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func coverage.out

# Generate HTML coverage report
go tool cover -html coverage.out -o coverage.html

# View coverage percentages directly
go test -cover ./...
```

## Test Coverage Areas

### Current Coverage Status

| Package | Coverage | Status | Lines of Code | Weighted Impact |
|---------|----------|--------|---------------|-----------------|
| `shared` | 79.4% | High | ~235 | Small |
| `config` | 79.5% | High | ~523 | Small |
| `cmd/server` | 4.2% | Low | ~342 | Small |
| `plugin/store` | 46.8% | Medium | ~494 | Medium |
| `plugin/host` | 20.9% | Low | ~412 | Medium |
| `plugin/manager` | 12.4% | Low | ~383 | Medium |
| `server` | 10.7% | Low | ~4300 | Large |
| `client` | 0% | None | ~4500 | Large |
| `plugin/license` | 0% | None | ~188 | Small |

**Overall coverage: 9.3%** (all packages)

### High Coverage (70%+)
- **Shared Package**: Cryptographic operations, data types, message handling
- **Config Package**: Configuration loading, validation, environment variables

### Medium Coverage (40-70%)
- **Plugin Store**: Registry management, platform resolution, filtering, caching (46.8%)

### Low Coverage (<40%)
- **Plugin Host**: Plugin lifecycle management, communication, enable/disable (20.9%)
- **Plugin Manager**: Installation, uninstallation, command execution (12.4%)
- **Server Package**: Basic database operations, user management (10.7%)
- **Server Main**: Flag parsing, configuration validation (4.2%)
- **Client Package**: No test files currently exist (0%)
- **Plugin License**: No test files currently exist (0%)

### Areas for Future Testing
- **Server Package**: WebSocket handling, message routing, admin panel (current: 10.7%)
- **Client Package**: Configuration loading, crypto operations, file picker (current: 0%)
- **Plugin Host**: Live plugin execution, WebSocket communication (current: 20.9%)
- **Plugin Manager**: Installation, uninstallation, command execution (current: 12.4%)
- **Server Main**: Full main function execution, server startup, admin panel integration (current: 4.2%)
- **File Transfer**: File upload/download functionality
- **Plugin License**: License validation and enforcement (current: 0%)

## Test Data and Fixtures

### Database Tests
- Uses in-memory SQLite databases for isolation
- Creates fresh schema for each test
- Tests both encrypted and plaintext messages
- Verifies message ordering and retrieval

### Cryptographic Tests
- Generates real keypairs using X25519
- Tests ChaCha20-Poly1305 encryption
- Verifies session key derivation
- Tests key validation and ID generation

### Message Tests
- Tests various message types (text, file, admin)
- Verifies JSON serialization/deserialization
- Tests message ordering and timestamp handling
- Validates encrypted message handling

### Server Main Tests
- Tests command-line flag parsing and validation
- Verifies multi-flag functionality for admin users
- Tests configuration loading and environment variable handling
- Validates TLS configuration and WebSocket scheme selection
- Tests admin username normalization and duplicate detection
- Verifies banner display functionality
- Tests deprecated flag warnings and backward compatibility

## Continuous Integration

The test suite is designed to run in CI/CD environments:

- **Fast Execution**: Most tests complete in under 1 second
- **No External Dependencies**: Uses only Go standard library and SQLite
- **Parallel Safe**: Tests can run concurrently
- **Deterministic**: No flaky tests or race conditions

## Adding New Tests

### Guidelines

1. **Test Naming**: Use descriptive test names that explain the scenario
2. **Test Structure**: Follow the Arrange-Act-Assert pattern
3. **Isolation**: Each test should be independent and not rely on other tests
4. **Coverage**: Aim for meaningful coverage, not just line coverage
5. **Documentation**: Add comments for complex test scenarios

### Example Test Structure

```go
func TestFeatureName(t *testing.T) {
    // Arrange
    setup := createTestSetup()
    input := createTestInput()
    
    // Act
    result, err := functionUnderTest(input)
    
    // Assert
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    if result != expectedResult {
        t.Errorf("Expected %v, got %v", expectedResult, result)
    }
}
```

### Database Test Pattern

```go
func TestDatabaseOperation(t *testing.T) {
    // Create test database
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("Failed to open test database: %v", err)
    }
    defer db.Close()
    
    // Create schema
    CreateSchema(db)
    
    // Test the operation
    // ... test implementation
}
```

## Performance Considerations

- **In-Memory Databases**: Tests use SQLite in-memory mode for speed
- **Parallel Execution**: Tests are designed to run in parallel when possible
- **Minimal Setup**: Each test creates only the data it needs
- **Fast Cleanup**: Tests clean up resources immediately

## Troubleshooting

### Common Issues

1. **Import Errors**: Ensure all dependencies are properly imported
2. **Database Locks**: Tests use separate in-memory databases to avoid conflicts
3. **Race Conditions**: All concurrent tests use proper synchronization
4. **Memory Leaks**: Tests properly close database connections and channels
5. **PowerShell Execution Policy**: May need to enable script execution: `Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser`
6. **Coverage Tool Syntax**: Use `go tool cover -func coverage.out` (without equals sign)

### Debug Mode

Run tests with debug output:

```bash
# Enable Go test debugging
go test -v -race ./...
```

## Contributing

When adding new functionality to Marchat:

1. **Write Tests First**: Follow TDD principles where possible
2. **Update This Document**: Add new test categories and coverage areas
3. **Maintain Coverage**: Ensure new code is properly tested
4. **Run Full Suite**: Always run all tests before submitting changes

## Test Metrics

- **Total Tests**: 90+ individual test cases across 9 packages
- **Coverage by Package**: 79.5% (config), 79.4% (shared), 46.8% (plugin/store), 20.9% (plugin/host), 12.4% (plugin/manager), 10.7% (server), 4.2% (cmd/server), 0% (client, plugin/license)
- **Overall Coverage**: 9.3% across all packages
- **Execution Time**: <1 second for full suite
- **Reliability**: 100% deterministic, no flaky tests, no hanging tests
- **Test Files**: 10 test files covering core functionality, plugin system, and server startup

This foundational test suite provides a solid base for testing core functionality, with room for significant expansion in the main application components.
