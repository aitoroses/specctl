# specctl skill

Agent skill teaching specification governance and Agent-First verification.

Install: `npx skills add <owner>/specctl`

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

The resulting chain:

```
tools/specctl/skills/specctl/    ← canonical source (edit here)
  ↑ symlink
~/.agents/skills/specctl/        ← global install
  ↑ symlink
~/.claude/skills/specctl/        ← Claude Code reads from here
```

### Re-publishing

After edits, re-run `npx skills add ./skills -g -y` to update the copy
for agents that don't follow symlinks (Cursor, Codex, etc.).
