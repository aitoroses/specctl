# Changelog

This repository primarily records behavioral history inside its governed spec tracking files:

- `.specs/specctl/cli.yaml`
- `.specs/specctl/mcp.yaml`
- `.specs/specctl/dashboard.yaml`

Those files contain structured changelog entries tied to revision bumps.

## Unreleased

- Added repository docs (`README`, `ARCHITECTURE`, `CONTRIBUTING`)
- Tightened ignore/build hygiene for binaries, runtime state, and dashboard artifacts
- Introduced a narrow `specctl:mcp` transport subspec for MCP-specific behavior
