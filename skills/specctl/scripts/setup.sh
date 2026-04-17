#!/usr/bin/env bash
set -euo pipefail

# specctl setup — install binary + configure MCP
# Usage: bash scripts/setup.sh [--global]
#   --global: configure MCP in ~/.claude/.mcp.json (user-level)
#   default:  configure MCP in .mcp.json (project-level)

SPECCTL_REPO="github.com/aitoroses/specctl"
SPECCTL_CMD="cmd/specctl"
MCP_SCOPE="project"

for arg in "$@"; do
  case "$arg" in
    --global) MCP_SCOPE="global" ;;
  esac
done

# --- Step 1: Install specctl binary ---

if command -v specctl &>/dev/null; then
  echo "[specctl] Binary already installed: $(which specctl)"
  specctl --help | head -1
else
  echo "[specctl] Installing via go install..."
  if ! command -v go &>/dev/null; then
    echo "[specctl] ERROR: Go is not installed."
    echo "  Install Go: https://go.dev/dl/"
    echo "  Or download a prebuilt binary from the specctl releases page."
    exit 1
  fi
  go install "${SPECCTL_REPO}/${SPECCTL_CMD}@latest"
  echo "[specctl] Installed: $(which specctl)"
fi

# --- Step 2: Configure MCP ---

if [ "$MCP_SCOPE" = "global" ]; then
  MCP_FILE="${HOME}/.claude/.mcp.json"
  mkdir -p "$(dirname "$MCP_FILE")"
else
  MCP_FILE=".mcp.json"
fi

SPECCTL_BIN="$(which specctl)"

# Build the MCP entry
MCP_ENTRY=$(cat <<ENTRY
{
  "specctl": {
    "command": "${SPECCTL_BIN}",
    "args": ["mcp"],
    "type": "stdio"
  }
}
ENTRY
)

if [ -f "$MCP_FILE" ]; then
  # Check if specctl is already configured
  if command -v jq &>/dev/null; then
    if jq -e '.mcpServers.specctl // .specctl' "$MCP_FILE" &>/dev/null; then
      echo "[specctl] MCP already configured in $MCP_FILE"
    else
      # Merge into existing file
      EXISTING=$(cat "$MCP_FILE")
      if echo "$EXISTING" | jq -e '.mcpServers' &>/dev/null; then
        echo "$EXISTING" | jq --argjson entry "$MCP_ENTRY" '.mcpServers += $entry' > "$MCP_FILE"
      else
        echo "$EXISTING" | jq --argjson entry "$MCP_ENTRY" '. + {mcpServers: $entry}' > "$MCP_FILE"
      fi
      echo "[specctl] MCP configured in $MCP_FILE"
    fi
  else
    echo "[specctl] WARNING: jq not found. Add specctl to $MCP_FILE manually:"
    echo "$MCP_ENTRY"
  fi
else
  # Create new MCP file
  if command -v jq &>/dev/null; then
    echo "{\"mcpServers\": $(echo "$MCP_ENTRY" | jq '.')}" | jq '.' > "$MCP_FILE"
  else
    cat > "$MCP_FILE" <<EOF
{
  "mcpServers": {
    "specctl": {
      "command": "${SPECCTL_BIN}",
      "args": ["mcp"],
      "type": "stdio"
    }
  }
}
EOF
  fi
  echo "[specctl] MCP configured in $MCP_FILE"
fi

# --- Step 3: Verify ---

echo ""
echo "[specctl] Setup complete. Verify with:"
echo "  specctl example    # see a governed spec"
echo "  specctl --help     # all commands"
echo ""
echo "[specctl] Next steps:"
echo "  1. Run 'specctl example' to see the embedded reference spec"
echo "  2. For a new project:  specctl charter create <name>"
echo "  3. For an existing project: specctl context (to check current state)"
