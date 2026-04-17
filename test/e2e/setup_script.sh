#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/skills/specctl/scripts/setup.sh"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

FAKEBIN="${TMPDIR}/bin"
mkdir -p "$FAKEBIN"
cat > "${FAKEBIN}/specctl" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "--help" ]; then
  echo "specctl help"
else
  echo "specctl ${*:-}"
fi
EOF
chmod +x "${FAKEBIN}/specctl"

export PATH="${FAKEBIN}:$PATH"
export HOME="${TMPDIR}/home"
mkdir -p "${HOME}"

run_setup() {
  (
    cd "${TMPDIR}/repo"
    bash "${SCRIPT}" "$@"
  )
}

mkdir -p "${TMPDIR}/repo"

echo "[setup-test] project JSON config"
run_setup
python3 - "${TMPDIR}/repo/.mcp.json" <<'PY'
import json,sys
path = sys.argv[1]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
assert data["mcpServers"]["specctl"]["command"].endswith("/specctl")
assert data["mcpServers"]["specctl"]["args"] == ["mcp"]
assert data["mcpServers"]["specctl"]["type"] == "stdio"
print("project-json-ok")
PY

echo "[setup-test] stale JSON is repaired idempotently"
cat > "${TMPDIR}/repo/.mcp.json" <<'EOF'
{
  "foo": true,
  "specctl": {
    "command": "/old/path/specctl",
    "args": ["old"]
  }
}
EOF
run_setup
python3 - "${TMPDIR}/repo/.mcp.json" <<'PY'
import json,sys
path = sys.argv[1]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
assert "specctl" not in data
assert data["foo"] is True
entry = data["mcpServers"]["specctl"]
assert entry["args"] == ["mcp"]
assert entry["type"] == "stdio"
print("json-repair-ok")
PY
BEFORE="$(shasum -a 256 "${TMPDIR}/repo/.mcp.json" | awk '{print $1}')"
run_setup
AFTER="$(shasum -a 256 "${TMPDIR}/repo/.mcp.json" | awk '{print $1}')"
test "${BEFORE}" = "${AFTER}"

echo "[setup-test] codex global TOML config"
mkdir -p "${HOME}/.codex"
cat > "${HOME}/.codex/config.toml" <<'EOF'
model = "gpt-5"

[mcp_servers.other]
command = "other"
args = ["x"]
enabled = true
startup_timeout_sec = 5

[mcp_servers.specctl]
command = "/old/specctl"
args = ["old"]
enabled = false
startup_timeout_sec = 1
EOF
run_setup --codex-global
python3 - "${HOME}/.codex/config.toml" <<'PY'
import sys,tomllib
path = sys.argv[1]
with open(path, 'rb') as f:
    data = tomllib.load(f)
entry = data["mcp_servers"]["specctl"]
assert entry["args"] == ["mcp"]
assert entry["enabled"] is True
assert entry["startup_timeout_sec"] == 5
assert data["mcp_servers"]["other"]["command"] == "other"
print("codex-global-ok")
PY
BEFORE="$(shasum -a 256 "${HOME}/.codex/config.toml" | awk '{print $1}')"
run_setup --codex-global
AFTER="$(shasum -a 256 "${HOME}/.codex/config.toml" | awk '{print $1}')"
test "${BEFORE}" = "${AFTER}"

echo "[setup-test] global both"
run_setup --global
test -f "${HOME}/.claude/.mcp.json"
test -f "${HOME}/.codex/config.toml"
echo "setup-script-e2e-ok"
