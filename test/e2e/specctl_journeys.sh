#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
MODULE_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/specctl-e2e.XXXXXX")
SPECCTL_BIN="$TMP_ROOT/specctl"

cleanup() {
	rm -rf "$TMP_ROOT"
}
trap cleanup EXIT HUP INT TERM

run_specctl() {
	repo_root=$1
	stdin_payload=$2
	shift 2

	RUN_STDOUT=$(mktemp "$TMP_ROOT/stdout.XXXXXX")
	RUN_STDERR=$(mktemp "$TMP_ROOT/stderr.XXXXXX")
	if (
		cd "$repo_root" &&
		printf '%s' "$stdin_payload" | "$SPECCTL_BIN" "$@"
	) >"$RUN_STDOUT" 2>"$RUN_STDERR"; then
		RUN_STATUS=0
	else
		RUN_STATUS=$?
	fi
}

require_status() {
	expected=$1
	if [ "$RUN_STATUS" -ne "$expected" ]; then
		echo "expected exit $expected, got $RUN_STATUS" >&2
		echo "stdout:" >&2
		cat "$RUN_STDOUT" >&2
		echo "stderr:" >&2
		cat "$RUN_STDERR" >&2
		exit 1
	fi
}

require_empty_stderr() {
	if [ -s "$RUN_STDERR" ]; then
		echo "expected empty stderr" >&2
		cat "$RUN_STDERR" >&2
		exit 1
	fi
}

assert_json() {
	json_file=$1
	assertion_code=$(cat)
	ASSERT_JSON_CODE=$assertion_code python3 - "$json_file" <<'PY'
import json
import os
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    doc = json.load(handle)

code = os.environ["ASSERT_JSON_CODE"]
namespace = {"doc": doc}
exec(compile(code, "<assert_json>", "exec"), namespace, namespace)
PY
}

setup_lifecycle_repo() {
	repo_root=$1
	mkdir -p \
		"$repo_root/.specs" \
		"$repo_root/runtime/src/domain/session_execution" \
		"$repo_root/runtime/tests/domain" \
		"$repo_root/runtime/tests/e2e/journeys" \
		"$repo_root/runtime/src/orphan" \
		"$repo_root/docs"
cat >"$repo_root/runtime/src/domain/session_execution/SPEC.md" <<'EOF'
---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Requirement: Compensation cleanup

```gherkin requirement
@runtime
Feature: Compensation cleanup
```

### Scenarios

```gherkin scenario
Scenario: Cleanup runs after a failure
  Given stage 4 fails during compensation
  When recovery completes
  Then cleanup steps run in documented order
```

## Overview
Initial lifecycle notes.
EOF
}

setup_charter_order_repo() {
	repo_root=$1
	mkdir -p \
		"$repo_root/.specs" \
		"$repo_root/runtime/src/domain/redis_state" \
		"$repo_root/runtime/src/application/contracts" \
		"$repo_root/runtime/src/domain/session_execution"
}

init_git_repo() {
	repo_root=$1
	(
		cd "$repo_root"
		git init -q
		git config user.name "specctl e2e"
		git config user.email "specctl-e2e@example.com"
		git add .
		git commit --allow-empty -q -m "baseline"
		git rev-parse HEAD
	)
}

commit_all() {
	repo_root=$1
	message=$2
	(
		cd "$repo_root"
		git add .
		git commit -q -m "$message"
		git rev-parse HEAD
	)
}

echo "building specctl binary"
(
	cd "$MODULE_ROOT"
	go build -o "$SPECCTL_BIN" ./cmd/specctl
)

LIFECYCLE_REPO="$TMP_ROOT/lifecycle"
setup_lifecycle_repo "$LIFECYCLE_REPO"
init_git_repo "$LIFECYCLE_REPO" >/dev/null

echo "journey: lifecycle writes"
run_specctl "$LIFECYCLE_REPO" "title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: recovery
    title: Recovery and Cleanup
    order: 20
" charter create runtime
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "charter", doc
assert doc["state"]["name"] == "runtime", doc
assert doc["state"]["tracking_file"] == ".specs/runtime/CHARTER.yaml", doc
PY

run_specctl "$LIFECYCLE_REPO" "" spec create runtime:session-lifecycle --title "Session Lifecycle" --doc runtime/src/domain/session_execution/SPEC.md --scope runtime/src/domain/session_execution/ --group recovery --order 20 --charter-notes "Session FSM and cleanup behavior"
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "spec", doc
assert doc["state"]["slug"] == "session-lifecycle", doc
assert doc["state"]["status"] == "draft", doc
PY

run_specctl "$LIFECYCLE_REPO" "current: Failure cleanup is undocumented
target: Capture the compensation cleanup contract
notes: Needed before recovery work lands
" delta add runtime:session-lifecycle --intent add --area "Compensation stage 4"
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "delta", doc
assert doc["result"]["delta"]["id"] == "D-001", doc
assert doc["state"]["status"] == "ready", doc
PY

run_specctl "$LIFECYCLE_REPO" "@runtime
Feature: Compensation cleanup

  Scenario: Cleanup runs after a failure
    Given stage 4 fails during compensation
    When recovery completes
    Then cleanup steps run in documented order
" req add runtime:session-lifecycle --delta D-001
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "requirement", doc
assert doc["result"]["requirement"]["id"] == "REQ-001", doc
assert doc["state"]["status"] == "active", doc
titles = [item["title"] for item in doc["state"]["requirements"]]
assert "Compensation cleanup" in titles, doc
PY

cat >"$LIFECYCLE_REPO/runtime/tests/domain/test_compensation_cleanup.py" <<'EOF'
def test_cleanup():
    assert True
EOF

run_specctl "$LIFECYCLE_REPO" "" req verify runtime:session-lifecycle REQ-001 --test-file runtime/tests/domain/test_compensation_cleanup.py
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "requirement", doc
assert doc["result"]["requirement"]["verification"] == "verified", doc
PY

run_specctl "$LIFECYCLE_REPO" "" delta close runtime:session-lifecycle D-001
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["kind"] == "delta", doc
assert doc["result"]["delta"]["status"] == "closed", doc
assert doc["state"]["status"] == "verified", doc
PY

echo "journey: representative reads"
run_specctl "$LIFECYCLE_REPO" "" diff runtime:session-lifecycle
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["state"]["target"] == "runtime:session-lifecycle", doc
assert doc["state"]["baseline"] == "checkpoint", doc
assert doc["state"]["from"] is None, doc
assert doc["state"]["to"]["status"] == "verified", doc
assert doc["state"]["model"]["deltas"]["opened"] == [], doc
assert doc["state"]["model"]["deltas"]["closed"] == [], doc
assert doc["state"]["model"]["deltas"]["deferred"] == [], doc
assert doc["state"]["model"]["deltas"]["resumed"] == [], doc
assert doc["state"]["model"]["requirements"]["added"] == [], doc
assert doc["state"]["model"]["requirements"]["verified"] == [], doc
assert doc["state"]["model"]["spec_tags"]["added"] == [], doc
assert doc["state"]["model"]["scope"]["added"] == ["runtime/src/domain/session_execution/"], doc
PY

cat >"$LIFECYCLE_REPO/runtime/src/domain/session_execution/services.py" <<'EOF'
def service():
    return "ok"
EOF
cat >"$LIFECYCLE_REPO/runtime/src/orphan/worker.py" <<'EOF'
def worker():
    return "orphan"
EOF
cat >"$LIFECYCLE_REPO/docs/notes.md" <<'EOF'
# Notes
EOF

run_specctl "$LIFECYCLE_REPO" "runtime/src/domain/session_execution/services.py
runtime/src/orphan/worker.py
docs/notes.md
" hook
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["state"]["considered_files"] == [
    "runtime/src/domain/session_execution/services.py",
    "runtime/src/orphan/worker.py",
], doc
assert doc["state"]["ignored_files"] == ["docs/notes.md"], doc
assert doc["state"]["unmatched_files"] == ["runtime/src/orphan/worker.py"], doc
assert len(doc["state"]["affected_specs"]) == 1, doc
affected = doc["state"]["affected_specs"][0]
assert affected["slug"] == "session-lifecycle", doc
assert affected["matched_files"] == ["runtime/src/domain/session_execution/services.py"], doc
assert doc["state"]["validation"]["findings"][0]["code"] == "UNOWNED_SOURCE_FILE", doc
PY

echo "journey: revision bump"
HEAD_SHA=$(commit_all "$LIFECYCLE_REPO" "lifecycle baseline")
run_specctl "$LIFECYCLE_REPO" "Closed the compensation cleanup work and synced the design doc.
" rev bump runtime:session-lifecycle --checkpoint HEAD
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<PY
assert doc["result"]["kind"] == "revision", doc
assert doc["result"]["checkpoint"] == "$HEAD_SHA", doc
assert doc["state"]["rev"] == 2, doc
assert doc["state"]["checkpoint"] == "$HEAD_SHA", doc
assert doc["state"]["status"] == "verified", doc
PY

echo "journey: drift review and error recovery"
cat >"$LIFECYCLE_REPO/runtime/src/domain/session_execution/SPEC.md" <<'EOF'
---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Requirement: Compensation cleanup

```gherkin requirement
@runtime
Feature: Compensation cleanup
```

### Scenarios

```gherkin scenario
Scenario: Cleanup runs after a failure
  Given stage 4 fails during compensation
  When recovery completes
  Then cleanup steps run in documented order
```

## Requirement: Drift review cleanup

```gherkin requirement
@runtime @e2e
Feature: Drift review cleanup
```

### Scenarios

```gherkin scenario
Scenario: Review captures the recovery ordering
  Given the design doc drifted after the last checkpoint
  When the recovery journey is reviewed end to end
  Then cleanup ordering is captured and testable
```

## Overview
Initial lifecycle notes.

## Drift Review
Recovery ordering now needs explicit documentation.
EOF

run_specctl "$LIFECYCLE_REPO" "" diff runtime:session-lifecycle
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["state"]["baseline"] == "checkpoint", doc
assert doc["state"]["from"]["rev"] == 1, doc
assert doc["state"]["to"]["rev"] == 2, doc
assert doc["state"]["design_doc"]["changed"] is True, doc
sections = doc["state"]["design_doc"]["sections_changed"]
assert any(section["heading"] == "Drift Review" and section["type"] == "added" for section in sections), doc
PY

run_specctl "$LIFECYCLE_REPO" "current: Recovery ordering is implied but not reviewable
target: Document drift-review cleanup ordering and recovery steps
notes: Discovered while reviewing the post-checkpoint design doc
" delta add runtime:session-lifecycle --intent add --area "Drift review cleanup"
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["delta"]["id"] == "D-002", doc
assert doc["state"]["status"] == "ready", doc
PY

run_specctl "$LIFECYCLE_REPO" "@runtime @e2e
Feature: Drift review cleanup

  Scenario: Review captures the recovery ordering
    Given the design doc drifted after the last checkpoint
    When the recovery journey is reviewed end to end
    Then cleanup ordering is captured and testable
" req add runtime:session-lifecycle --delta D-002
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["requirement"]["id"] == "REQ-002", doc
assert doc["state"]["status"] == "active", doc
assert doc["result"]["requirement"]["tags"] == ["runtime", "e2e"], doc
PY

run_specctl "$LIFECYCLE_REPO" "" delta close runtime:session-lifecycle D-002
require_status 1
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["error"]["code"] == "UNVERIFIED_REQUIREMENTS", doc
assert doc["focus"]["delta"]["id"] == "D-002", doc
blocking = doc["focus"]["blocking_requirements"]
assert len(blocking) == 1, doc
assert blocking[0]["id"] == "REQ-002", doc
assert blocking[0]["verification"] == "unverified", doc
assert doc["next"]["mode"] == "sequence", doc
assert doc["next"]["steps"][0]["action"] == "verify_requirement", doc
assert doc["next"]["steps"][0]["template"]["argv"][4] == "REQ-002", doc
PY

cat >"$LIFECYCLE_REPO/runtime/tests/e2e/journeys/test_drift_review_cleanup.py" <<'EOF'
def test_drift_review_cleanup():
    assert True
EOF

run_specctl "$LIFECYCLE_REPO" "" req verify runtime:session-lifecycle REQ-002 --test-file runtime/tests/e2e/journeys/test_drift_review_cleanup.py
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["requirement"]["id"] == "REQ-002", doc
assert doc["result"]["requirement"]["verification"] == "verified", doc
PY

run_specctl "$LIFECYCLE_REPO" "" delta close runtime:session-lifecycle D-002
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
assert doc["result"]["delta"]["id"] == "D-002", doc
assert doc["result"]["delta"]["status"] == "closed", doc
assert doc["state"]["status"] == "verified", doc
PY

DRIFT_HEAD=$(commit_all "$LIFECYCLE_REPO" "drift review changes")
run_specctl "$LIFECYCLE_REPO" "Captured the drift review recovery ordering and its E2E journey.
" rev bump runtime:session-lifecycle --checkpoint HEAD
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<PY
assert doc["result"]["kind"] == "revision", doc
assert doc["result"]["checkpoint"] == "$DRIFT_HEAD", doc
assert doc["state"]["rev"] == 3, doc
assert doc["state"]["checkpoint"] == "$DRIFT_HEAD", doc
assert doc["state"]["status"] == "verified", doc
PY

echo "journey: charter dependency ordering"
ORDER_REPO="$TMP_ROOT/charter-order"
setup_charter_order_repo "$ORDER_REPO"
init_git_repo "$ORDER_REPO" >/dev/null

run_specctl "$ORDER_REPO" "title: Runtime System
description: Specs for runtime control-plane and data-plane behavior
groups:
  - key: execution
    title: Execution Engine
    order: 10
  - key: recovery
    title: Recovery and Cleanup
    order: 20
" charter create runtime
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "" spec create runtime:redis-state --title "Redis State" --doc runtime/src/domain/redis_state/SPEC.md --scope runtime/src/domain/redis_state/ --group execution --order 10 --charter-notes "Storage and CAS guarantees"
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "" spec create runtime:runtime-api-contract --title "Runtime API Contract" --doc runtime/src/application/contracts/SPEC.md --scope runtime/src/application/contracts/ --group recovery --order 20 --charter-notes "HTTP request and response contract"
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "" spec create runtime:session-lifecycle --title "Session Lifecycle" --doc runtime/src/domain/session_execution/SPEC.md --scope runtime/src/domain/session_execution/ --group recovery --order 30 --charter-notes "Session FSM and cleanup behavior"
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "group: recovery
order: 20
depends_on:
  - redis-state
notes: HTTP request and response contract
" charter add-spec runtime runtime-api-contract
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "group: recovery
order: 30
depends_on:
  - redis-state
notes: Session FSM and cleanup behavior
" charter add-spec runtime session-lifecycle
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "group: recovery
order: 30
depends_on:
  - runtime-api-contract
notes: Session FSM and cleanup behavior
" charter add-spec runtime session-lifecycle
require_status 0
require_empty_stderr

run_specctl "$ORDER_REPO" "" context runtime
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
ordered = doc["state"]["ordered_specs"]
assert [item["slug"] for item in ordered] == ["redis-state", "runtime-api-contract", "session-lifecycle"], doc
assert ordered[0]["depends_on"] == [], doc
assert ordered[1]["depends_on"] == ["redis-state"], doc
assert ordered[2]["depends_on"] == ["runtime-api-contract"], doc
PY

run_specctl "$ORDER_REPO" "" diff --charter runtime
require_status 0
require_empty_stderr
assert_json "$RUN_STDOUT" <<'PY'
ordered = doc["state"]["ordered_specs"]
assert [item["slug"] for item in ordered] == ["redis-state", "runtime-api-contract", "session-lifecycle"], doc
assert ordered[0]["depends_on"] == [], doc
assert ordered[1]["depends_on"] == ["redis-state"], doc
assert ordered[2]["depends_on"] == ["runtime-api-contract"], doc
assert all(item["from"] is None for item in ordered), doc
assert all(item["changed"] is True for item in ordered), doc
PY

echo "specctl shell journeys passed"
