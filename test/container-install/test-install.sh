#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Containerized Installation & Configuration Smoke Test Harness
# Validates install.sh, CLI bootstrap, and sndbx doctor inside an isolated rootless container.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
IMAGE="${1:-docker.io/library/debian:bookworm-slim}"

echo "=========================================================="
echo " Starting Containerized Installation Smoke Test"
echo " Target Image: ${IMAGE}"
echo "=========================================================="

CONTAINER_NAME="test-install-smoke-$$-${RANDOM}"

cleanup() {
  echo "==> Cleaning up test container ${CONTAINER_NAME}..."
  podman rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# 1. Run container with isolated unprivileged test user
podman run -d --name "${CONTAINER_NAME}" \
  -v "${REPO_DIR}:/repo:ro" \
  "${IMAGE}" \
  sleep 300 >/dev/null

echo "==> Preparing clean environment inside container..."
podman exec "${CONTAINER_NAME}" bash -c "
  set -euo pipefail
  apt-get update -qq >/dev/null
  apt-get install -y -qq git curl openssh-client ca-certificates tar gzip >/dev/null
  useradd -m -s /bin/bash tester
  # Install modern Go toolchain matching host / base image
  curl -fsSL "https://go.dev/dl/go1.24.1.linux-amd64.tar.gz" | tar -C /usr/local -xz
  echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/profile
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  # Provide mock podman inside container if needed by preflight checks
  cat << 'MOCK' > /usr/local/bin/podman
#!/bin/bash
if [ \"\$1\" = \"--version\" ]; then
  echo \"podman version 4.9.3 (mock-container-harness)\"
  exit 0
fi
echo \"mock podman called with: \$*\"
exit 0
MOCK
  chmod +x /usr/local/bin/podman
"

echo "==> Executing install.sh inside container as unprivileged user 'tester'..."
podman exec -u tester "${CONTAINER_NAME}" bash -c "
  set -euo pipefail
  export HOME=/home/tester
  export USER=tester
  export PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin
  cd /home/tester
  # Copy local repo to simulate clean checkout
  mkdir -p /home/tester/agent-sandbox
  cp -r /repo/. /home/tester/agent-sandbox/
  cd /home/tester/agent-sandbox

  # Run installer
  ./install.sh

  # Verify XDG paths and files
  test -f /home/tester/.local/bin/sndbx
  test -f /home/tester/.local/bin/bp
  test -f /home/tester/.local/share/agent-sandbox/.env
  test -f /home/tester/.ssh/agent-sandbox
  test -f /home/tester/.ssh/agent-sandbox.pub

  # Verify file permissions
  test \"\$(stat -c '%a' /home/tester/.local/share/agent-sandbox/.env)\" = \"600\"
  test \"\$(stat -c '%a' /home/tester/.ssh/agent-sandbox)\" = \"600\"

  # Test CLI execution from outside repo directory
  cd /home/tester
  export PATH=\"/home/tester/.local/bin:\$PATH\"
  sndbx --help >/dev/null
  bp --help >/dev/null

  # Verify repo path resolution works
  RESOLVED_PATH=\"\$(sndbx repo path)\"
  echo \"Resolved repo path: \$RESOLVED_PATH\"
  test \"\$RESOLVED_PATH\" = \"/home/tester/agent-sandbox\"
"

echo "=========================================================="
echo " Containerized Installation Smoke Test Passed!"
echo "=========================================================="
