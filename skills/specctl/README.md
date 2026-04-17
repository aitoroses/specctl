# specctl skill

Agent skill teaching specification governance and Agent-First verification.

Install:

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

Then run the packaged setup path depending on your client:

```bash
# Project-local .mcp.json
bash skills/specctl/scripts/setup.sh

# Claude Code global MCP config
bash skills/specctl/scripts/setup.sh --claude-global

# Codex global MCP config
bash skills/specctl/scripts/setup.sh --codex-global

# Both Claude + Codex global config
bash skills/specctl/scripts/setup.sh --global
```

## Development

After editing skill files, changes propagate instantly if installed via symlink.

### Local symlink install

```bash
# Install globally (creates a copy)
npx skills add ./skills -g -y

# Replace the copy with a symlink for live editing
rm -rf ~/.agents/skills/specctl
ln -s "$(pwd)/skills/specctl" ~/.agents/skills/specctl
```

The resulting chain for local development:

```
skills/specctl/                  ← canonical source (edit here)
  ↑ symlink
~/.agents/skills/specctl/        ← global install
  ↑ symlink
~/.claude/skills/specctl/        ← Claude Code reads from here
```

### Re-publishing

After edits, re-run `npx skills add ./skills -g -y` to update the copy
for agents that don't follow symlinks (Cursor, Codex, etc.).

## Idempotence

The setup script is intended to be safe to re-run:

- existing `specctl` MCP entries are updated in place if stale
- unrelated config is preserved
- legacy JSON layout is normalized to `.mcpServers.specctl`
- Codex TOML config is updated without duplicating the `[mcp_servers.specctl]` block
