#!/usr/bin/env bash
set -euo pipefail

# specctl setup — install binary + configure MCP
#
# Targets:
#   default          -> project-local .mcp.json
#   --global         -> ~/.claude/.mcp.json + ~/.codex/config.toml
#   --claude-global  -> ~/.claude/.mcp.json
#   --codex-global   -> ~/.codex/config.toml
#   --codex-project  -> ./.codex/config.toml
#   --project        -> ./.mcp.json

SPECCTL_REPO="github.com/aitoroses/specctl"
SPECCTL_CMD="cmd/specctl"

TARGET_MODE="project-json"

for arg in "$@"; do
  case "$arg" in
    --global) TARGET_MODE="global-both" ;;
    --claude-global) TARGET_MODE="claude-global" ;;
    --codex-global) TARGET_MODE="codex-global" ;;
    --codex-project) TARGET_MODE="codex-project" ;;
    --project) TARGET_MODE="project-json" ;;
    *)
      echo "[specctl] ERROR: unknown argument: $arg"
      exit 1
      ;;
  esac
done

resolve_specctl_bin() {
  if command -v specctl >/dev/null 2>&1; then
    command -v specctl
    return 0
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "[specctl] ERROR: Go is not installed."
    echo "  Install Go: https://go.dev/dl/"
    echo "  Or install a released specctl binary."
    exit 1
  fi

  echo "[specctl] Installing via go install..."
  go install "${SPECCTL_REPO}/${SPECCTL_CMD}@latest"

  local gobin gopath candidate
  gobin="$(go env GOBIN)"
  gopath="$(go env GOPATH)"

  if [ -n "$gobin" ]; then
    candidate="$gobin/specctl"
    if [ -x "$candidate" ]; then
      echo "$candidate"
      return 0
    fi
  fi

  candidate="$gopath/bin/specctl"
  if [ -x "$candidate" ]; then
    echo "$candidate"
    return 0
  fi

  echo "[specctl] ERROR: specctl was installed but could not be resolved on PATH, GOBIN, or GOPATH/bin."
  echo "  Add the Go bin directory to PATH, then re-run setup."
  exit 1
}

atomic_write() {
  local target="$1"
  local content="$2"
  local dir tmp
  dir="$(dirname "$target")"
  mkdir -p "$dir"
  tmp="$(mktemp "${dir}/.specctl-setup.XXXXXX")"
  printf '%s' "$content" > "$tmp"
  mv "$tmp" "$target"
}

configure_json_mcp() {
  local target="$1"
  local specctl_bin="$2"
  local normalized

  normalized="$(python3 - "$target" "$specctl_bin" <<'PY'
import json
import os
import sys
from copy import deepcopy

target = sys.argv[1]
specctl_bin = sys.argv[2]

desired = {
    "command": specctl_bin,
    "args": ["mcp"],
    "type": "stdio",
}

if os.path.exists(target):
    with open(target, "r", encoding="utf-8") as f:
        raw = f.read().strip()
    if raw:
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as e:
            print(f"[specctl] ERROR: invalid JSON in {target}: {e}", file=sys.stderr)
            sys.exit(2)
    else:
        data = {}
else:
    data = {}

if not isinstance(data, dict):
    print(f"[specctl] ERROR: expected top-level JSON object in {target}", file=sys.stderr)
    sys.exit(2)

before = deepcopy(data)
legacy = data.pop("specctl", None)
servers = data.get("mcpServers")
if not isinstance(servers, dict):
    servers = {}
data["mcpServers"] = servers

current = servers.get("specctl")
if current != desired:
    servers["specctl"] = desired

changed = before != data or legacy is not None
print(json.dumps({"changed": changed, "content": json.dumps(data, indent=2) + "\n"}))
PY
)"

  if [ -z "$normalized" ]; then
    echo "[specctl] ERROR: failed to normalize JSON MCP config for $target"
    exit 1
  fi

  local changed content
  changed="$(printf '%s' "$normalized" | python3 -c 'import json,sys; print(json.load(sys.stdin)["changed"])')"
  content="$(printf '%s' "$normalized" | python3 -c 'import json,sys; print(json.load(sys.stdin)["content"], end="")')"

  if [ "$changed" = "True" ]; then
    atomic_write "$target" "$content"
    echo "[specctl] MCP configured in $target"
  else
    echo "[specctl] MCP already up to date in $target"
  fi
}

configure_codex_toml() {
  local target="$1"
  local specctl_bin="$2"
  local normalized

  normalized="$(python3 - "$target" "$specctl_bin" <<'PY'
import os
import sys
import tomllib

target = sys.argv[1]
specctl_bin = sys.argv[2]

block = (
    "[mcp_servers.specctl]\n"
    f'command = "{specctl_bin}"\n'
    'args = ["mcp"]\n'
    "enabled = true\n"
    "startup_timeout_sec = 5\n"
)

if os.path.exists(target):
    with open(target, "r", encoding="utf-8") as f:
        text = f.read()
    if text.strip():
        try:
            tomllib.loads(text)
        except tomllib.TOMLDecodeError as e:
            print(f"[specctl] ERROR: invalid TOML in {target}: {e}", file=sys.stderr)
            sys.exit(2)
else:
    text = ""

def strip_specctl_block(src: str) -> str:
    lines = src.splitlines()
    kept = []
    skipping = False
    for line in lines:
        if line.strip() == "[mcp_servers.specctl]":
            skipping = True
            continue
        if skipping:
            if line.startswith("[") and line.endswith("]"):
                skipping = False
                kept.append(line)
            else:
                continue
        else:
            kept.append(line)
    return "\n".join(kept).strip()

stripped = strip_specctl_block(text)
if stripped:
    new_text = stripped + "\n\n" + block
else:
    new_text = block

tomllib.loads(new_text)
changed = (text != new_text)
print("1" if changed else "0")
print(new_text, end="")
PY
)"

  local changed content
  changed="$(printf '%s' "$normalized" | head -n 1)"
  content="$(printf '%s' "$normalized" | tail -n +2)"

  if [ "$changed" = "1" ]; then
    atomic_write "$target" "$content"
    echo "[specctl] Codex MCP configured in $target"
  else
    echo "[specctl] Codex MCP already up to date in $target"
  fi
}

SPECCTL_BIN="$(resolve_specctl_bin)"
echo "[specctl] Binary ready: ${SPECCTL_BIN}"

case "$TARGET_MODE" in
  project-json)
    configure_json_mcp ".mcp.json" "$SPECCTL_BIN"
    ;;
  claude-global)
    configure_json_mcp "${HOME}/.claude/.mcp.json" "$SPECCTL_BIN"
    ;;
  codex-global)
    configure_codex_toml "${HOME}/.codex/config.toml" "$SPECCTL_BIN"
    ;;
  codex-project)
    configure_codex_toml ".codex/config.toml" "$SPECCTL_BIN"
    ;;
  global-both)
    configure_json_mcp "${HOME}/.claude/.mcp.json" "$SPECCTL_BIN"
    configure_codex_toml "${HOME}/.codex/config.toml" "$SPECCTL_BIN"
    ;;
  *)
    echo "[specctl] ERROR: unknown target mode: $TARGET_MODE"
    exit 1
    ;;
esac

echo ""
echo "[specctl] Setup complete. Verify with:"
echo "  specctl example    # see a governed spec"
echo "  specctl --help     # all commands"
echo ""
echo "[specctl] Next steps:"
echo "  1. Run 'specctl example' to see the embedded reference spec"
echo "  2. For a new project:  specctl charter create <name>"
echo "  3. For an existing project: specctl context (to check current state)"
