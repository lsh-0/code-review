#!/usr/bin/env bash
# manage stuf

set -e

GO_VERSION="go1.19.13"

check_go_version() {
    if ! command -v "$GO_VERSION" &> /dev/null; then
        echo "Error: $GO_VERSION not found"
        echo ""
        echo "Please install Go 1.19.13:"
        echo "  go install golang.org/dl/go1.19.13@latest"
        echo "  go1.19.13 download"
        exit 1
    fi
}

cmd_build() {
    cmd_gopher_build
    cmd_wails_build
}

cmd_gopher_build() {
    check_go_version
    local gopherjs_bin
    gopherjs_bin="$(go env GOPATH)/bin/gopherjs"

    if [ ! -f "$gopherjs_bin" ]; then
        echo "Error: GopherJS not found at $gopherjs_bin"
        echo "Run './manage.sh setup' first"
        exit 1
    fi

    echo "Building review.js..."
    cd frontend
    GOWORK=off GOPHERJS_GOROOT=$("$GO_VERSION" env GOROOT) "$gopherjs_bin" build --source_map=false -o ../assets/review.js
    cd ..
    echo "Build complete: review.js"
}

cmd_wails_build() {
    echo "Building Wails application..."
    cd backend
    wails build 2>&1 | grep -v "If Wails is useful" | grep -v "github.com/sponsors"
    cd ..
    echo "Build complete: backend/build/bin/code-review"
}

cmd_wails_dev() {
    echo "Running Wails in development mode..."
    cd backend
    wails dev
}

cmd_release() {
    cmd_clean
    echo "Building release..."
    cmd_gopher_build

    echo ""
    read -p "Enter version (required): " version_input
    if [ -z "$version_input" ]; then
        echo "Error: Version is required for release builds"
        exit 1
    fi

    echo "Building Wails application for release (version: $version_input)..."
    cd backend
    wails build -clean -ldflags "-s -w -X 'main.version=$version_input'" 2>&1 | grep -v "If Wails is useful" | grep -v "github.com/sponsors"
    cd ..

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
    rm -rf wailsjs
    rm -rf .coverage
    rm -f assets/review.js
    echo "Clean complete"
}

cmd_lint() {
    echo "Running go fmt..."
    for dir in model frontend backend; do
        echo "  Formatting $dir..."
        (cd "$dir" && go fmt ./...)
    done
    echo "Lint complete"
}

cmd_gopher_test() {
    check_go_version
    local gopherjs_bin
    gopherjs_bin="$(go env GOPATH)/bin/gopherjs"

    if [ ! -f "$gopherjs_bin" ]; then
        echo "Error: GopherJS not found at $gopherjs_bin"
        echo "Run './manage.sh setup' first"
        exit 1
    fi

    echo "Running frontend tests..."
    cd frontend
    GOWORK=off GOPHERJS_GOROOT=$("$GO_VERSION" env GOROOT) "$gopherjs_bin" test -v
    cd ..
}

cmd_test() {
    check_go_version
    local gopherjs_bin
    gopherjs_bin="$(go env GOPATH)/bin/gopherjs"

    if [ ! -f "$gopherjs_bin" ]; then
        echo "Error: GopherJS not found at $gopherjs_bin"
        echo "Run './manage.sh setup' first"
        exit 1
    fi

    echo "Running unit tests with coverage..."
    echo ""

    local coverage_dir=".coverage"
    rm -rf "$coverage_dir"
    mkdir -p "$coverage_dir"

    local model_coverage="$coverage_dir/model.out"
    local backend_coverage="$coverage_dir/backend.out"
    local merged_coverage="$coverage_dir/merged.out"

    local model_exit=0
    local backend_exit=0
    local frontend_exit=0

    echo "=== Model Tests ==="
    cd model
    GOWORK=off go test -v -coverprofile="../$model_coverage" ./...
    model_exit=$?
    cd ..
    echo ""

    echo "=== Backend Tests ==="
    cd backend
    go test -v -coverprofile="../$backend_coverage" ./...
    backend_exit=$?
    cd ..
    echo ""

    echo "=== Frontend Tests ==="
    cd frontend
    GOWORK=off GOPHERJS_GOROOT=$("$GO_VERSION" env GOROOT) "$gopherjs_bin" test -v
    frontend_exit=$?
    cd ..
    echo ""

    echo "=== Coverage Report ==="
    echo ""

    if [ -f "$model_coverage" ]; then
        echo "Model coverage:"
        cd model
        GOWORK=off go tool cover -func="../$model_coverage" | tail -n 1
        cd ..
    else
        echo "Model coverage: N/A"
    fi

    if [ -f "$backend_coverage" ]; then
        echo "Backend coverage:"
        go tool cover -func="$backend_coverage" | tail -n 1
    else
        echo "Backend coverage:  N/A"
    fi

    if [ -f "$model_coverage" ] && [ -f "$backend_coverage" ]; then
        echo "mode: atomic" > "$merged_coverage"
        tail -n +2 "$model_coverage" >> "$merged_coverage"
        tail -n +2 "$backend_coverage" >> "$merged_coverage"
        echo ""
        echo "Overall coverage:"
        go tool cover -func="$merged_coverage" | tail -n 1
        echo ""
        echo "Merged coverage saved to: $merged_coverage"
    elif [ -f "$model_coverage" ]; then
        cp "$model_coverage" "$merged_coverage"
    elif [ -f "$backend_coverage" ]; then
        cp "$backend_coverage" "$merged_coverage"
    fi

    if [ -f "$merged_coverage" ]; then
        go tool cover -html="$merged_coverage" -o="$coverage_dir/coverage.html"
        echo "HTML coverage report: $coverage_dir/coverage.html"
    fi

    echo ""
    echo "Coverage files saved in: $coverage_dir/"

    if [ $model_exit -ne 0 ] || [ $backend_exit -ne 0 ] || [ $frontend_exit -ne 0 ]; then
        exit 1
    fi
}

cmd_setup() {
    echo "Checking system dependencies..."

    case "$(uname -s)" in
        Linux*)
            if ! pkg-config --exists webkit2gtk-4.0; then
                echo "Error: webkit2gtk-4.0 not found"
                echo ""
                echo "Please install WebKit2GTK:"
                echo "  sudo apt-get install libwebkit2gtk-4.0-dev"
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

    echo "Setting up Go 1.19.13 and GopherJS..."

    if ! command -v "$GO_VERSION" &> /dev/null; then
        echo "Installing Go 1.19.13 wrapper..."
        go install golang.org/dl/go1.19.13@latest
    fi

    echo "Downloading Go 1.19.13..."
    "$GO_VERSION" download

    echo "Installing GopherJS v1.19.0-beta2..."
    "$GO_VERSION" install github.com/gopherjs/gopherjs@v1.19.0-beta2

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
    setup              Install Go 1.19.13 and GopherJS
    build              Build frontend and Wails application
    gopher.build       Build only the JavaScript file using GopherJS
    gopher.test        Run frontend unit tests
    wails.build        Build only the Wails application
    wails.dev          Run Wails in development mode with dev tools
    test               Run all unit tests with coverage reporting
    lint               Format all Go code with go fmt
    release            Build optimised release binary to dist/
    release.install    Build release and install to ~/.local/bin
    release.install --system
                       Build release and install to /usr/local/bin
    clean              Remove build artifacts
    help               Show this help message

Note: This project requires Go 1.19.13 for GopherJS compatibility.
Run './manage.sh setup' to install the required tools.

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
        gopher.build)
            cmd_gopher_build
            ;;
        gopher.test)
            cmd_gopher_test
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
