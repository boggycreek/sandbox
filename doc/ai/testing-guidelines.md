# Testing Guidelines & Coverage Gates

## Strict Coverage Threshold (>= 90.0%)
- Monorepo enforces **>= 90.0% atomic statement coverage** across all packages in \`pkg/...\` and \`cmd/...\`.
- Gate command: \`make test-coverage\` (outputs HTML to \`coverage/coverage.html\`).
- When introducing new packages or functions, corresponding unit tests must be added to maintain or improve coverage.

## Rootless Podman Test Cleanups
- Rootless Podman creates files in test containers using subuid ranges.
- Standard \`t.TempDir()\` cleanups will fail with permission errors unless cleaned via \`podman unshare\`.
- Always register a cleanup hook in tests using temp dirs with Podman:
  \`\`\`go
  tmpDir := t.TempDir()
  t.Cleanup(func() {
      _ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
  })
  \`\`\`

## Containerized Smoke Testing
- \`make test-install\`: Runs isolated installation and bootstrap smoke testing inside an ephemeral rootless container (\`test/container-install/test-install.sh\`) without mutating host environment.

Further reading:
- [ADR 00018 — Quality Engineering & SCA](../adr/00018-QualityEngineeringTestingAndSCA.md)
- [ADR 00028 — Ephemeral Integration Test Lifecycles](../adr/00028-EphemeralIntegrationTestLifecycleAndIsolation.md)
