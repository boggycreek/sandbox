# Development Workflow & Pull Request Protocol

## Branching & Protection Rules
- \`main\` is protected against direct pushes and deletions.
- All code changes must go through a feature branch (\`feat/...\`, \`fix/...\`, \`test/...\`, \`docs/...\`).
- Changes must be merged via Pull Requests using \`gh pr create\` or GitHub UI.
- Never force-push to shared branches.

## Architectural Decision Records (ADRs)
- Significant design, CLI interface changes, or protocol modifications require an ADR in \`doc/adr/\`.
- Format follows \`NNNNN-TitleInCamelCase.md\` with sections: Context, Decision, Status, Consequences.
- Add new ADR entries to \`doc/adr/README.md\`.

## Quality Gates Checklist Before PR
Every branch must satisfy all quality gates before submission:
1. \`make format\`: Auto-format all Go source files.
2. \`make lint\`: Check Go files (\`golangci-lint\`) and shell scripts (\`shellcheck\`).
3. \`make test-coverage\`: Verify strict **>= 90.0% statement coverage** across all packages.
4. \`make check\`: Run full quality gate suite (lint + test-coverage + sca).
5. All new files must include the standard Boggy Creek Software LLC MIT copyright header.
