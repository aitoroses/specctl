# Releasing specctl

`specctl` is an **agent-facing tool**. Releases should preserve:

- the CLI binary surface
- the MCP transport surface
- the packaged skill under `skills/specctl/`
- the self-governed example surface used by `specctl example`

The dashboard code may continue to evolve in the repo, but it is not treated as a required public launch surface unless explicitly added back to the release contract.

## Versioning policy

Use semantic versioning.

- `MAJOR`: breaking workflow or public surface changes
- `MINOR`: backwards-compatible features
- `PATCH`: backwards-compatible fixes

## Public install contract

### Binary / Go install

Preferred install form:

```bash
go install github.com/aitoroses/specctl/cmd/specctl@latest
```

After the first public tagged release, prefer versioned installs in docs/examples:

```bash
go install github.com/aitoroses/specctl/cmd/specctl@vX.Y.Z
```

### Skill install

Public packaged skill install command:

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

This command must remain placeholder-free in public docs.

## Release checklist

1. Run local verification:
   - `pnpm --dir dashboard install --frozen-lockfile`
   - `pnpm --dir dashboard run build`
   - `go test ./...`
2. Verify shipped workflow:
   - `specctl example`
   - `specctl mcp`
   - representative `specctl context ...`
3. Verify packaged skill path:
   - install the public skill
   - run its shipped setup path
4. Confirm embedded assets exist:
   - `.specs/specctl.yaml`
   - `.specs/specctl/CHARTER.yaml`
   - `.specs/specctl/cli.yaml`
5. Update docs if public behavior changed.
6. Tag release.
7. Publish release notes.

## Human approval boundary

Even if issue work is primarily agent-driven:

- merges require human approval
- releases require human approval

Record repo settings evidence in `docs/oss-launch-ops.md`.
