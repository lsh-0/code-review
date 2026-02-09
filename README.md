# Code Review Tool

A desktop-first code review tool built with Go, GopherJS, and Wails. This tool provides a diff view of changes between your current branch and the main/master branch, allowing you to annotate lines with markdown comments that persist locally.

## Features

- View diffs between current branch and main/master branch
- Add, edit, and delete comments on any line
- Mark comments as resolved or ignored
- Reactivate resolved/ignored comments
- State persists to XDG data directory
- Works entirely offline
- No dependency on GitHub or other remote services

## Requirements

- Go 1.24 or later
- Go 1.19.13 (for GopherJS compatibility)
- GopherJS v1.19.0-beta2
- Wails v2
- WebKit2GTK (Linux only)

## Setup

Install dependencies:

```bash
./manage.sh setup
```

This will install Go 1.19.13, GopherJS, and Wails.

## Building

Build the application:

```bash
./manage.sh build
```

This builds both the GopherJS frontend and the Wails backend.

## Running

The tool must be invoked from within a git repository:

```bash
cd /path/to/your/git/repo
/path/to/code-review/backend/build/bin/code-review
```

Or install it:

```bash
./manage.sh release.install
```

Then run from any git repository:

```bash
cd /path/to/your/git/repo
code-review
```

## Testing

Run all tests with coverage:

```bash
./manage.sh test
```

Run specific test suites:

```bash
./manage.sh gopher.test  # Frontend tests only
cd backend && go test    # Backend tests only
cd model && go test      # Model tests only
```

## Development

Run in development mode with hot reload:

```bash
./manage.sh wails.dev
```

Format code:

```bash
./manage.sh lint
```

Clean build artifacts:

```bash
./manage.sh clean
```

## Architecture

The project follows a clean architecture with clear separation:

- `model/` - Core data structures (Review, Comment, FileDiff)
- `backend/` - Wails backend application with git integration and diff parsing
- `frontend/` - GopherJS frontend for UI rendering and interaction
- `assets/` - HTML, CSS, and embedded assets
- `manage.sh` - Build and development tooling

## State Persistence

Review state is automatically saved to:
- Linux: `$XDG_DATA_HOME/code-review/` or `~/.local/share/code-review/`

Each repository has its own state file based on the repository path.

## License

This is a template project for building desktop applications with Go and Wails.
