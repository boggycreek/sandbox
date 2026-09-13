#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Agent Sandbox — Contributor & Developer Environment Setup Script
#
# Configures local development tools, Go dependencies, C build environments,
# and developer hooks across macOS and Linux platforms.
#
# Usage:
#   ./dev-setup.sh [--dry-run]

set -euo pipefail

DRY_RUN=0
if [ "${1:-}" = "--dry-run" ] || [ "${1:-}" = "-n" ]; then
  DRY_RUN=1
fi

echo "======================================================"
echo "    Agent Sandbox — Developer Environment Setup       "
echo "======================================================"
echo

# 1. OS & Architecture Detection
OS_TYPE="$(uname -s)"
ARCH_TYPE="$(uname -m)"

case "${OS_TYPE}" in
  Darwin)
    OS="darwin"
    OS_PRETTY="macOS"
    ;;
  Linux)
    OS="linux"
    if [ -f /etc/os-release ]; then
      # shellcheck disable=SC1091
      DISTRO_NAME="$(. /etc/os-release && echo "${PRETTY_NAME:-Linux}")"
      DISTRO_ID="$(. /etc/os-release && echo "${ID:-linux}")"
      OS_PRETTY="Linux (${DISTRO_NAME})"
    else
      DISTRO_ID="linux"
      OS_PRETTY="Linux"
    fi
    ;;
  *)
    echo "Error: Unsupported development platform: ${OS_TYPE}" >&2
    exit 1
    ;;
esac

echo "Detected Platform: ${OS_PRETTY} [${OS}/${ARCH_TYPE}]"
echo

# Helper functions for checking tools
MISSING_CORE=()
MISSING_DEV=()
MISSING_OPTIONAL=()

check_cmd() {
  local cmd="$1"
  local name="${2:-$1}"
  local category="${3:-core}"

  local target="${cmd}"
  if ! command -v "${target}" >/dev/null 2>&1; then
    local gopath_bin
    gopath_bin="$(go env GOPATH 2>/dev/null || true)/bin/${cmd}"
    if [ -n "${gopath_bin}" ] && [ -x "${gopath_bin}" ]; then
      target="${gopath_bin}"
    fi
  fi

  if command -v "${target}" >/dev/null 2>&1 || [ -x "${target}" ]; then
    local version
    version="$("${target}" --version 2>/dev/null | head -n1 || echo "installed")"
    printf "  [✓] %-20s : %s\n" "${name}" "${version}"
  else
    printf "  [✗] %-20s : NOT FOUND\n" "${name}"
    if [ "${category}" = "core" ]; then
      MISSING_CORE+=("${name}")
    elif [ "${category}" = "dev" ]; then
      MISSING_DEV+=("${name}")
    else
      MISSING_OPTIONAL+=("${name}")
    fi
  fi
}

# 2. Inspecting Core Tooling
echo "--- [1/4] Core Tooling ---"
check_cmd "git" "Git" "core"
check_cmd "curl" "cURL" "core"
check_cmd "make" "GNU Make" "core"
check_cmd "podman" "Podman (Engine)" "core"

# 3. Inspecting Go & Native Toolchains
echo
echo "--- [2/4] Go & Native C Toolchains ---"
check_cmd "go" "Go Compiler" "core"
check_cmd "gcc" "C Compiler (GCC/Clang)" "dev"
check_cmd "pkg-config" "pkg-config" "dev"

# Linters, SCA & Security Analysis Tools
check_cmd "golangci-lint" "golangci-lint" "dev"
check_cmd "shellcheck" "ShellCheck" "dev"
check_cmd "govulncheck" "govulncheck (SCA)" "dev"
check_cmd "gosec" "gosec (AST Security)" "dev"
check_cmd "deadcode" "deadcode (Reachability)" "dev"

# GUI Development Toolchains
echo
echo "--- [3/4] GUI Development Toolchains ---"
if [ "${OS}" = "darwin" ]; then
  check_cmd "swift" "Swift Toolchain" "optional"
elif [ "${OS}" = "linux" ]; then
  # Check for GTK4 development libraries via pkg-config
  if command -v pkg-config >/dev/null 2>&1; then
    for pkg in gtk4 libadwaita-1 librsvg-2.0; do
      if pkg-config --exists "${pkg}" >/dev/null 2>&1; then
        pkg_ver="$(pkg-config --modversion "${pkg}")"
        printf "  [✓] %-20s : installed (v%s)\n" "${pkg} (C dev library)" "${pkg_ver}"
      else
        printf "  [✗] %-20s : NOT FOUND\n" "${pkg} (C dev library)"
        MISSING_OPTIONAL+=("${pkg}-dev")
      fi
    done
  fi
fi

# 4. Dependency Installation Recommendations
echo
echo "--- [4/4] Setup Actions & Guidance ---"

if [ ${#MISSING_CORE[@]} -ne 0 ] || [ ${#MISSING_DEV[@]} -ne 0 ]; then
  echo "Missing developer packages detected."
  echo "Run the following package manager command for your system:"
  echo

  if [ "${OS}" = "darwin" ]; then
    echo "  # macOS (Homebrew):"
    echo "  brew install go podman podman-compose make pkg-config golangci-lint shellcheck"
    echo "  go install golang.org/x/vuln/cmd/govulncheck@latest"
    echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"
    echo "  go install golang.org/x/tools/cmd/deadcode@latest"
  elif [ "${OS}" = "linux" ]; then
    case "${DISTRO_ID:-}" in
      ubuntu|debian|pop)
        echo "  # Ubuntu / Debian / Pop!_OS:"
        echo "  sudo apt update && sudo apt install -y golang-go podman build-essential pkg-config shellcheck libgtk-4-dev libadwaita-1-dev librsvg2-dev"
        echo "  go install golang.org/x/vuln/cmd/govulncheck@latest"
        echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"
        echo "  go install golang.org/x/tools/cmd/deadcode@latest"
        echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin"
        ;;
      fedora|rhel)
        echo "  # Fedora / RHEL:"
        echo "  sudo dnf install -y golang podman gcc make pkgconf-pkg-config gtk4-devel libadwaita-devel librsvg2-devel"
        ;;
      arch|manjaro)
        echo "  # Arch Linux / Manjaro:"
        echo "  sudo pacman -S --needed go podman base-devel pkgconf gtk4 libadwaita librsvg"
        ;;
      *)
        echo "  Install equivalent packages for: go, podman, gcc, make, pkg-config, gtk4, libadwaita"
        ;;
    esac
  fi
  echo
fi

if [ "${DRY_RUN}" -eq 1 ]; then
  echo "Check completed (--dry-run). No configuration changes made."
  exit 0
fi

# 5. Initialize Local Development Environment
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${REPO_ROOT}"

echo "Initializing repository development configurations..."

# Create local .env if missing in repo root for dev overrides
if [ ! -f "${REPO_ROOT}/.env" ]; then
  echo "  Creating developer .env template..."
  cat > "${REPO_ROOT}/.env" <<EOF
# Local Development Environment Overrides
HUMAN_NAME=${USER:-developer}
ADMIN_BACKPLANE_PASSWORD=dev_admin_password_insecure
HUMAN_BACKPLANE_PASSWORD=dev_human_password_insecure

# Backplane testing defaults
BP_HOST=localhost
BP_PORT=6379
EOF
  chmod 600 "${REPO_ROOT}/.env"
fi

# Ensure XDG target directories exist
DATA_HOME="${XDG_DATA_HOME:-${HOME}/.local/share}/agent-sandbox"
mkdir -p "${DATA_HOME}/secrets"
mkdir -p "${HOME}/.local/bin"

# Download Go module dependencies if Go is installed and go.mod exists
if command -v go >/dev/null 2>&1 && [ -f "${REPO_ROOT}/go.mod" ]; then
  echo "  Fetching Go dependencies (go mod download)..."
  go mod download || true
fi

# Configure Git local hooks directory if exists
if [ -d "${REPO_ROOT}/.githooks" ]; then
  echo "  Configuring git core.hooksPath -> .githooks..."
  git config core.hooksPath .githooks
fi

echo
echo "======================================================"
echo "    Developer Environment Setup Complete!            "
echo "======================================================"
echo
echo "Ready to build and develop:"
echo "  - Build CLI:         make build-cli   (or go build -o bin/ ./cmd/...)"
echo "  - Run Tests:         make test        (or go test ./...)"
echo "  - Start Local Infra: make infra-up    (or sndbx infra up)"
echo
