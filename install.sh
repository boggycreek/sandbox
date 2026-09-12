#!/usr/bin/env bash
# Agent Sandbox Installer
#
# Supports macOS (Darwin) and Linux (all standard distributions).
#
# Can be executed via curl from GitHub:
#   curl -fsSL https://raw.githubusercontent.com/boggycreek/agent-sandbox/main/install.sh | bash
#
# Or run directly from a cloned repository checkout:
#   ./install.sh
#
# Options:
#   --remote <url>    Override the git remote repository URL to clone from.
#   --branch <branch> Specify a git branch or tag (default: main).
#   -h, --help        Show usage information.

set -euo pipefail

# Default configuration
REPO_URL_DEFAULT="https://github.com/boggycreek/agent-sandbox.git"
DEFAULT_BRANCH="main"

DATA_HOME="${XDG_DATA_HOME:-${HOME}/.local/share}/agent-sandbox"
REPO_DIR="${DATA_HOME}/repo"
BIN_DIR="${HOME}/.local/bin"
SECRETS_DIR="${DATA_HOME}/secrets"

REMOTE_URL="${AGENT_SANDBOX_REMOTE:-${REPO_URL_DEFAULT}}"
BRANCH="${AGENT_SANDBOX_BRANCH:-${DEFAULT_BRANCH}}"

print_usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --remote <url>     Override repository remote URL
  --branch <branch>  Specify repository branch/tag to checkout (default: main)
  -h, --help         Show this help message
EOF
}

# Parse command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --remote)
      REMOTE_URL="$2"
      shift 2
      ;;
    --branch)
      BRANCH="$2"
      shift 2
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Error: Unknown option: $1" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

echo "========================================"
echo "       Agent Sandbox Installer         "
echo "========================================"
echo

# 1. Operating System & Architecture Detection
echo "[1/6] Detecting operating system and hardware platform..."
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
      # Source os-release for pretty distro name
      # shellcheck disable=SC1091
      DISTRO_NAME="$(. /etc/os-release && echo "${PRETTY_NAME:-Linux}")"
      OS_PRETTY="Linux (${DISTRO_NAME})"
    else
      OS_PRETTY="Linux"
    fi
    ;;
  *)
    echo "Error: Unsupported operating system: ${OS_TYPE}." >&2
    echo "Agent Sandbox currently supports macOS (Darwin) and Linux." >&2
    exit 1
    ;;
esac

case "${ARCH_TYPE}" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported CPU architecture: ${ARCH_TYPE}." >&2
    echo "Supported architectures: x86_64 / amd64, arm64 / aarch64." >&2
    exit 1
    ;;
esac

echo "  Platform: ${OS_PRETTY} [${OS}/${ARCH}]"

# 2. Preflight Dependency Checks
echo
echo "[2/6] Checking host prerequisites..."
MISSING_TOOLS=()

for tool in git curl ssh-keygen podman; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    MISSING_TOOLS+=("${tool}")
  fi
done

if [ ${#MISSING_TOOLS[@]} -ne 0 ]; then
  echo "Error: The following required utilities are missing: ${MISSING_TOOLS[*]}" >&2
  if [ "${OS}" = "darwin" ]; then
    echo "On macOS:" >&2
    echo "  - Install Xcode Command Line Tools: xcode-select --install" >&2
    echo "  - Install Podman: brew install podman (or Podman Desktop)" >&2
  elif [ "${OS}" = "linux" ]; then
    echo "On Linux, install via your package manager:" >&2
    echo "  - Ubuntu/Debian: sudo apt update && sudo apt install -y git curl openssh-client podman" >&2
    echo "  - Fedora/RHEL:   sudo dnf install -y git curl openssh podman" >&2
    echo "  - Arch Linux:    sudo pacman -S --needed git curl openssh podman" >&2
  fi
  exit 1
fi

CONTAINER_ENGINE="podman"
echo "  Container engine: podman ($(${CONTAINER_ENGINE} --version | head -n1))"

# Detect Go Toolchain
if command -v go >/dev/null 2>&1; then
  echo "  Go compiler detected: $(go version)"
else
  echo "  Note: Go compiler not found on PATH. Required for building native binaries from source."
fi

# 3. Setup XDG Directories
echo
echo "[3/6] Configuring XDG directories..."
mkdir -p "${BIN_DIR}"
mkdir -p "${DATA_HOME}"
mkdir -p "${SECRETS_DIR}"

echo "  Binaries directory : ${BIN_DIR}"
echo "  Data directory     : ${DATA_HOME}"

# 4. Sync / Resolve Repository Location
echo
echo "[4/6] Setting up agent-sandbox repository..."

# Determine if we are running from an existing local clone or via piped curl
CURRENT_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"

if [ -n "${CURRENT_SCRIPT_DIR}" ] && [ -d "${CURRENT_SCRIPT_DIR}/.git" ]; then
  # Executing directly from within a local git repository clone
  SANDBOX_ROOT="${CURRENT_SCRIPT_DIR}"
  echo "  Running from local repository checkout at: ${SANDBOX_ROOT}"
else
  # Executing via curl pipe or outside a repository checkout -> ensure standard XDG checkout
  if [ -d "${REPO_DIR}/.git" ]; then
    echo "  Existing repository checkout found at ${REPO_DIR}. Syncing..."
    if ! git -C "${REPO_DIR}" diff --quiet || ! git -C "${REPO_DIR}" diff --cached --quiet; then
      echo "  Warning: ${REPO_DIR} has local modifications. Skipping git pull to preserve changes."
    else
      git -C "${REPO_DIR}" pull --ff-only || echo "  Warning: git pull failed. Continuing with existing checkout."
    fi
  else
    echo "  Cloning ${REMOTE_URL} (branch: ${BRANCH}) into ${REPO_DIR}..."
    git clone --branch "${BRANCH}" "${REMOTE_URL}" "${REPO_DIR}"
  fi
  SANDBOX_ROOT="${REPO_DIR}"
fi

# 5. Generate Dedicated IDE SSH Keypair
echo
echo "[5/6] Ensuring dedicated IDE SSH keypair..."
SSH_KEY="${HOME}/.ssh/agent-sandbox"
mkdir -p "${HOME}/.ssh"
chmod 700 "${HOME}/.ssh"

if [ ! -f "${SSH_KEY}" ]; then
  echo "  Generating dedicated IDE keypair: ${SSH_KEY} (ed25519)"
  ssh-keygen -t ed25519 -N "" -C "agent-sandbox-ide" -f "${SSH_KEY}" >/dev/null
  chmod 600 "${SSH_KEY}"
  chmod 644 "${SSH_KEY}.pub"
else
  echo "  Existing IDE keypair found at ${SSH_KEY}"
fi

# 6. Initialize Environment and CLI Entrypoint
echo
echo "[6/6] Initializing configuration and host CLI..."

ENV_FILE="${DATA_HOME}/.env"
if [ ! -f "${ENV_FILE}" ]; then
  echo "  Creating initial configuration at ${ENV_FILE}"
  HUMAN_NAME="${USER:-operator}"

  # Generate random 32-character hex secret portably across macOS and Linux
  if command -v openssl >/dev/null 2>&1; then
    ADMIN_PW="$(openssl rand -hex 16)"
    HUMAN_PW="$(openssl rand -hex 16)"
  else
    ADMIN_PW="$(od -vN 16 -An -tx1 /dev/urandom | tr -d ' \n')"
    HUMAN_PW="$(od -vN 16 -An -tx1 /dev/urandom | tr -d ' \n')"
  fi

  cat > "${ENV_FILE}" <<EOF
# Agent Sandbox Environment Configuration
# Generated for ${OS_PRETTY} (${OS}/${ARCH})

HUMAN_NAME=${HUMAN_NAME}
ADMIN_BACKPLANE_PASSWORD=${ADMIN_PW}
HUMAN_BACKPLANE_PASSWORD=${HUMAN_PW}

# Optional API Keys (uncomment and populate as needed)
# ANTHROPIC_API_KEY=
# OPENAI_API_KEY=
# GH_TOKEN=
EOF
  chmod 600 "${ENV_FILE}"
else
  echo "  Existing configuration file found at ${ENV_FILE}"
fi

# Configure sndbx CLI wrapper/executable
SNDBX_BIN="${BIN_DIR}/sndbx"
if [ ! -f "${SNDBX_BIN}" ]; then
  cat > "${SNDBX_BIN}" <<'EOF'
#!/usr/bin/env bash
# Temporary bootstrap dispatcher until native Go binary is compiled
echo "Agent Sandbox CLI (sndbx)"
echo "Run 'sndbx help' for commands once built, or build via: cd ~/.local/share/agent-sandbox/repo && make build-cli"
EOF
  chmod +x "${SNDBX_BIN}"
fi

echo
echo "========================================"
echo "       Installation Succeeded!         "
echo "========================================"
echo
echo "Target Platform     : ${OS_PRETTY} [${OS}/${ARCH}]"
echo "Repository Location : ${SANDBOX_ROOT}"
echo "Environment Config  : ${ENV_FILE}"
echo "CLI Executable      : ${SNDBX_BIN}"
echo "IDE SSH Public Key  : ${SSH_KEY}.pub"
echo

# Determine user's active shell configuration file
USER_SHELL="$(basename "${SHELL:-bash}")"
case "${USER_SHELL}" in
  zsh)
    SHELL_RC="${HOME}/.zshrc"
    ;;
  bash)
    if [ "${OS}" = "darwin" ]; then
      SHELL_RC="${HOME}/.bash_profile"
    else
      SHELL_RC="${HOME}/.bashrc"
    fi
    ;;
  *)
    SHELL_RC="${HOME}/.profile"
    ;;
esac

# PATH verification reminder
case ":${PATH}:" in
  *:"${BIN_DIR}":*) ;;
  *)
    echo "Notice: ${BIN_DIR} is not currently in your \$PATH."
    echo "Add it by running the following command:"
    echo
    echo "  echo 'export PATH=\"${BIN_DIR}:\$PATH\"' >> \"${SHELL_RC}\""
    echo "  source \"${SHELL_RC}\""
    echo
    ;;
esac

echo "Next steps:"
echo "  1. (Optional) Edit API keys in: ${ENV_FILE}"
echo "  2. Navigate to repo: cd \"${SANDBOX_ROOT}\""
echo "  3. Build binaries: make build-cli"
echo
