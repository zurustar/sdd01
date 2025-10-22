# Enterprise Scheduler

A comprehensive meeting room and schedule management system built with Go.

## Features

- 🏢 **Multi-user Support**: Role-based access control for employees and administrators
- 📅 **Schedule Management**: Create, update, and manage meeting schedules
- 🏠 **Room Booking**: Meeting room reservation and management
- 🔄 **Recurring Events**: Support for recurring meeting patterns
- 🔐 **Authentication**: Secure session-based authentication
- 📊 **SQLite Backend**: Reliable data persistence with migration support

## Quick Start

### Prerequisites

- Go 1.20 or later
- Make (optional, for using Makefile commands)

### Installation

```bash
# Clone the repository
git clone https://github.com/example/enterprise-scheduler.git
cd enterprise-scheduler

# Install dependencies
go mod download

# Build the application
make build
# or
go build -o bin/scheduler ./cmd/scheduler
```

### Running the Application

```bash
# Run directly
./bin/scheduler

# Or using go run
go run ./cmd/scheduler

# Development mode with auto-reload
make dev
```

## Development

### Development Tools

Install required development tools:

```bash
make install-tools
```

This installs:
- `golangci-lint` for code linting
- `goimports` for import formatting
- Additional security scanning tools

### Code Quality

Run all quality checks:

```bash
make quality
```

This runs:
- Code formatting (`gofmt`, `goimports`)
- Linting (`golangci-lint`)
- Vet analysis (`go vet`)
- Tests with race detection
- Coverage threshold check (80%)

### Testing

```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run tests without CGO (for deployment verification)
make test-cgo-disabled

# Generate coverage report
make coverage

# Check coverage threshold
make coverage-check
```

### Building

```bash
# Build for current platform
make build

# Build with CGO disabled (for static linking)
make build-cgo-disabled

# Build for multiple platforms
make build-all
```

### Linting

```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix
```

## CI/CD

This project uses GitHub Actions for continuous integration:

### Workflows

- **Lint**: Code quality checks using `golangci-lint`
- **Test**: Unit tests with race detection across Go versions
- **Build**: Multi-platform builds with CGO disabled
- **Coverage**: Coverage reporting with 80% threshold

### Coverage Requirements

- Minimum coverage threshold: **80%**
- Coverage is checked on every PR and push
- HTML coverage reports are generated for detailed analysis

### Build Matrix

Tests run on:
- Go 1.20
- Go 1.21
- Ubuntu Latest

Builds for:
- Linux AMD64
- macOS AMD64  
- Windows AMD64

## Database

### Migrations

The application uses SQLite with automatic migrations:

```bash
# Run migrations only
make migrate
```

### Schema

The database schema includes:
- `users` - User accounts and authentication
- `rooms` - Meeting room catalog
- `schedules` - Meeting schedules
- `schedule_participants` - Meeting participants
- `recurrences` - Recurring meeting rules
- `sessions` - Authentication sessions

## Configuration

Configuration is handled through environment variables:

```bash
# Database
DATABASE_PATH=./scheduler.db

# Server
PORT=8080
HOST=localhost

# Logging
LOG_LEVEL=info
```

## API Documentation

The application provides a REST API for all operations:

- `POST /sessions` - Authentication
- `GET /schedules` - List schedules
- `POST /schedules` - Create schedule
- `GET /rooms` - List rooms
- `POST /rooms` - Create room (admin only)

## Security

- Session-based authentication
- Role-based access control
- SQL injection prevention
- Input validation and sanitization
- Secure password hashing with Argon2id

## Performance

- SQLite with WAL mode for concurrent access
- Connection pooling and transaction management
- Efficient query patterns with proper indexing
- Caching for frequently accessed data

## Deployment

### Docker

```bash
# Build Docker image
make docker-build

# Run container
make docker-run
```

### Static Binary

For deployment in minimal environments:

```bash
# Build static binary (no CGO dependencies)
make build-cgo-disabled

# Verify no dynamic dependencies
./scripts/test-cgo-disabled.sh
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run quality checks: `make quality`
5. Submit a pull request

### Code Standards

- Follow Go conventions and idioms
- Maintain test coverage above 80%
- Use meaningful commit messages
- Add documentation for public APIs
- Run `make quality` before submitting

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For questions and support:
- Create an issue on GitHub
- Check the documentation in the `docs/` directory
- Review the API specification in `docs/enterprise_scheduler_spec.md`