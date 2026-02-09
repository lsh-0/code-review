# Code Review Tool

Local-first code review tool that allows per-line annotations on diffs betweens current branch and the main/master branch.

Annotations are preserved between branches and revisions.

Uses Go, GopherJS and Wails.

Final binary is small (<10MB) but with many system dependencies.

## Installation

Pre-requisites:

* WebKit2GTK (linux)
* WebKit (mac)


```bash
./manage.sh setup   # Installs Go 1.19.13, GopherJS, and Wails
./manage.sh build
cp ./backend/build/bin/code-review /path/to/your/local/bin/
```

## Running

From inside a git repo:

    code-review

## Development

```bash
./manage.sh wails.dev  # Hot reload development mode
./manage.sh test       # Run tests with coverage
./manage.sh clean      # Remove build artifacts
```

