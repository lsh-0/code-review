# Code Review Tool

Local-first code review tool that allows per-line annotations on diffs betweens current branch and the main/master branch.

Annotations are preserved between branches and revisions.

Uses Go and Wails for the backend; a TypeScript frontend built with Deno.

Final binary is small (<10MB) but with many system dependencies.

## Installation

Pre-requisites:

* WebKit2GTK (linux)
* WebKit (mac)
* [Deno](https://deno.com) (build-time only; not required to run the binary)


```bash
./manage.sh setup   # Checks WebKit and Deno, installs the Wails CLI
./manage.sh build
cp ./backend/build/bin/code-review /path/to/your/local/bin/

# or

./manage.sh release.install

# then

code-review
```

## Running

From inside a git repo:

    code-review

## Development

```bash
./manage.sh wails.dev    # Hot reload development mode
./manage.sh deno.bundle  # Rebuild the frontend bundle (assets/review.js)
./manage.sh deno.test    # Run the frontend (TypeScript) tests
./manage.sh test         # Run all tests (Go + frontend) with coverage
./manage.sh clean        # Remove build artifacts
```

