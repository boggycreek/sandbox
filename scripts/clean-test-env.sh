#!/usr/bin/env bash
# Copyright (c) 2026 Boggy Creek Software LLC
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Deterministic cleanup for test containers, orphaned conmon/slirp processes,
# and stale rootless network namespaces.

set -euo pipefail

UID_NUM="$(id -u)"
echo "==> Cleaning test environment for user ${UID_NUM}..."

# 1. Terminate orphaned test containers if any exist
if command -v podman >/dev/null 2>&1; then
  # Remove test-valkey-* or test-podman-* or infra-test-* containers
  TEST_CONTAINERS=$(podman ps -a --format '{{.Names}}' 2>/dev/null | grep -E '^(test-valkey-|test-podman-|infra-test-|test-gitea-)' || true)
  if [ -n "${TEST_CONTAINERS}" ]; then
    echo "Stopping test containers: ${TEST_CONTAINERS}"
    echo "${TEST_CONTAINERS}" | xargs -r podman stop -t 2 2>/dev/null || true
    echo "${TEST_CONTAINERS}" | xargs -r podman rm -f 2>/dev/null || true
  fi

  # Remove test networks and volumes
  TEST_NETS=$(podman network ls --format '{{.Name}}' 2>/dev/null | grep -E '^(test-|infra-test-)' || true)
  if [ -n "${TEST_NETS}" ]; then
    echo "Removing test networks: ${TEST_NETS}"
    echo "${TEST_NETS}" | xargs -r podman network rm -f 2>/dev/null || true
  fi

  TEST_VOLS=$(podman volume ls --format '{{.Name}}' 2>/dev/null | grep -E '^(test-|infra-test-)' || true)
  if [ -n "${TEST_VOLS}" ]; then
    echo "Removing test volumes: ${TEST_VOLS}"
    echo "${TEST_VOLS}" | xargs -r podman volume rm -f 2>/dev/null || true
  fi
fi

# 2. Terminate orphaned conmon, slirp4netns, and rootlessport test processes
# Only target processes owned by this user
pkill -9 -u "${UID_NUM}" -f "test-valkey|test-podman|infra-test" 2>/dev/null || true

# 3. Clean up stale netns descriptors and rootless-netns lockfiles in /run/user/<uid>
NETNS_DIR="/run/user/${UID_NUM}/netns"
if [ -d "${NETNS_DIR}" ]; then
  echo "Clearing stale test netns entries in ${NETNS_DIR}..."
  find "${NETNS_DIR}" -maxdepth 1 -name "netns-*" -type f -delete 2>/dev/null || true
fi

LIBPOD_TMP="/run/user/${UID_NUM}/libpod/tmp"
if [ -d "${LIBPOD_TMP}" ]; then
  echo "Clearing rootless netns locks in ${LIBPOD_TMP}..."
  rm -f "${LIBPOD_TMP}"/rootless-netns*.lock 2>/dev/null || true
  rm -f "${LIBPOD_TMP}"/rootless-netns*.pid 2>/dev/null || true
fi

echo "==> Test environment clean."
