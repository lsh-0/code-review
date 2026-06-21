#!/usr/bin/env bash
set -e

# build against webkit2gtk-4.1 (libsoup3). the older webkit2gtk-4.0 packages
# have been dropped from the Arch repos and only survive in the AUR, where they
# no longer track ICU; 4.1 is the maintained, ICU-current package.
WAILS_TAGS="webkit2_41"

# the frontend is TypeScript, built with Deno: type-checked, tested, and bundled
# to assets/review.js, which the Go binary embeds and serves. Deno is a
# build/test-time tool only and never ships in the binary.
cmd_deno_check() {
    echo "Type-checking TypeScript frontend..."
    deno task check
}

cmd_deno_test() {
    echo "Testing TypeScript frontend..."
    deno task test
}

cmd_deno_bundle() {
    echo "Bundling TypeScript frontend to assets/review.js..."
    deno task bundle
    echo "Bundle complete: assets/review.js"
}

cmd_build() {
    cmd_deno_bundle
    cmd_wails_build
}

cmd_wails_build() {
    echo "Building Wails application..."
    (
        cd backend
        wails build -tags "$WAILS_TAGS" 2>&1 | grep -v "If Wails is useful" | grep -v "github.com/sponsors"
    )
    echo "Build complete: backend/build/bin/code-review"
}

cmd_wails_dev() {
    echo "Running Wails in development mode..."
    (
        cd backend
        wails dev -tags "$WAILS_TAGS"
    )
}

cmd_release() {
    cmd_clean
    echo "Building release..."
    cmd_deno_bundle

    echo ""
    read -r -p "Enter version [unreleased]: " version_input
    if [ -z "$version_input" ]; then
        version_input="unreleased"
    fi

    echo "Building Wails application for release (version: $version_input)..."
    (
        cd backend
        wails build \
            -clean \
            -tags "$WAILS_TAGS" \
            -ldflags "-s -w -X 'main.version=$version_input'" 2>&1 | grep -v "If Wails is useful" | grep -v "github.com/sponsors"
    )

    echo "Copying binary to dist..."
    mkdir -p dist
    mv backend/build/bin/code-review dist/
    rm -rf backend/build/

    echo "Release complete: dist/code-review (version: $version_input)"
}

cmd_release_install() {
    local install_dir
    local use_sudo=""

    if [ -n "${2:-}" ] && [ "${2}" != "--system" ]; then
        echo "Error: Invalid argument '$2'"
        echo "Usage: ./manage.sh release.install [--system]"
        exit 1
    fi

    cmd_release

    if [ "${2:-}" = "--system" ]; then
        install_dir="/usr/local/bin"
        use_sudo="sudo --askpass"
        echo ""
        echo "Installing to system location: $install_dir"
        sudo --askpass -v
    else
        install_dir="$HOME/.local/bin"
        echo ""
        echo "Installing to user location: $install_dir"

        if [ ! -d "$install_dir" ]; then
            echo "Creating $install_dir..."
            mkdir -p "$install_dir"
        fi
    fi

    echo "Installing code-review..."
    $use_sudo install -m 755 dist/code-review "$install_dir/code-review"

    echo ""
    echo "Installation complete: $install_dir/code-review"

    if [ "${2:-}" != "--system" ] && [[ ":$PATH:" != *":$install_dir:"* ]]; then
        echo ""
        echo "Note: $install_dir is not in your PATH"
        echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
}

cmd_clean() {
    echo "Cleaning build artifacts..."
    rm -rf backend/build
    rm -rf dist
    rm -rf .coverage
    echo "Clean complete"
}

cmd_lint() {
    # the Go side is one module rooted at backend/ (model and assets are
    # packages within it), so a single tidy/fix/fmt/vet covers everything.
    echo "Linting Go..."
    (
        cd backend
        go mod tidy
        go fix ./...
        go fmt ./...
        go vet ./...
    )
    # frontend: TypeScript, formatted and linted by Deno.
    echo "Linting frontend (Deno)..."
    deno fmt
    deno lint

    echo "Lint complete"
}

cmd_test() {
    echo "Running unit tests with coverage..."
    echo ""

    local coverage_dir=".coverage"
    rm -rf "$coverage_dir"
    mkdir -p "$coverage_dir"

    local go_coverage="$coverage_dir/go.out"
    local go_exit=0
    local frontend_exit=0

    # one Go module rooted at backend/ covers all packages (backend, model,
    # assets) in a single run and a single coverage profile.
    echo "=== Go Tests ==="
    (
        cd backend
        go test -v -coverprofile="../$go_coverage" ./...
    )
    go_exit=$?
    echo ""

    echo "=== Frontend Tests (Deno) ==="
    deno task test
    frontend_exit=$?
    echo ""

    echo "=== Coverage Report ==="
    echo ""

    if [ -f "$go_coverage" ]; then
        echo "Go coverage:"
        (cd backend && go tool cover -func="../$go_coverage" | tail -n 1)
        go tool cover -html="$go_coverage" -o="$coverage_dir/coverage.html"
        echo "HTML coverage report: $coverage_dir/coverage.html"
    else
        echo "Go coverage: N/A"
    fi

    echo ""
    echo "Coverage files saved in: $coverage_dir/"

    if [ $go_exit -ne 0 ] || [ $frontend_exit -ne 0 ]; then
        exit 1
    fi
}

cmd_setup() {
    echo "Checking system dependencies..."

    case "$(uname -s)" in
        Linux*)
            if ! pkg-config --exists webkit2gtk-4.1; then
                echo "Error: webkit2gtk-4.1 not found"
                echo ""
                echo "Please install WebKit2GTK 4.1 (libsoup3):"
                echo "  Arch:   sudo pacman -S webkit2gtk-4.1"
                echo "  Debian: sudo apt-get install libwebkit2gtk-4.1-dev"
                echo ""
                exit 1
            fi
            ;;
        Darwin*)
            echo "macOS detected - using native WebKit"
            ;;
        *)
            echo "Warning: Unknown platform, skipping WebKit check"
            ;;
    esac

    if ! command -v deno &> /dev/null; then
        echo "Error: deno not found"
        echo ""
        echo "Please install Deno:"
        echo "  https://docs.deno.com/runtime/getting_started/installation/"
        echo ""
        exit 1
    fi

    echo "Installing Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest

    echo ""
    echo "Setup complete"
}

cmd_help() {
    cat <<EOF
Code Review Tool Management Script

Usage: ./manage.sh <command>

Commands:
    setup              Check WebKit, install the Wails CLI
    build              Bundle the frontend and build the Wails application
    deno.check         Type-check the TypeScript frontend
    deno.test          Test the TypeScript frontend
    deno.bundle        Bundle the TypeScript frontend to assets/review.js
    wails.build        Build only the Wails application
    wails.dev          Run Wails in development mode with dev tools
    test               Run all unit tests with coverage reporting
    lint               Tidy, format and vet the Go modules; fmt/lint the frontend
    release            Build optimised release binary to dist/
    release.install    Build release and install to ~/.local/bin
    release.install --system
                       Build release and install to /usr/local/bin
    clean              Remove build artifacts
    help               Show this help message

The frontend is TypeScript, built with Deno; the backend is Go with Wails.
Run './manage.sh setup' to check the required tools.

EOF
}

main() {
    case "${1:-}" in
        setup)
            cmd_setup
            ;;
        build)
            cmd_build
            ;;
        deno.check)
            cmd_deno_check
            ;;
        deno.test)
            cmd_deno_test
            ;;
        deno.bundle)
            cmd_deno_bundle
            ;;
        wails.build)
            cmd_wails_build
            ;;
        wails.dev)
            cmd_wails_dev
            ;;
        test)
            cmd_test
            ;;
        lint)
            cmd_lint
            ;;
        release)
            cmd_release
            ;;
        release.install)
            cmd_release_install "$@"
            ;;
        clean)
            cmd_clean
            ;;
        help|--help|-h)
            cmd_help
            ;;
        "")
            echo "Error: No command specified"
            echo ""
            cmd_help
            exit 1
            ;;
        *)
            echo "Error: Unknown command '$1'"
            echo ""
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
