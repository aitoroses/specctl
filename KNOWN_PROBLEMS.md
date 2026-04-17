# specctl Known Problems

Issues encountered by agents using specctl during dogfooding. Used to improve the CLI and its documentation.

---

## Fixed

### ~~TEST_FILES_REQUIRED gives no guidance about @manual~~ (Fixed)

**`req verify` without `--test-file` fails with no next steps**
- **What happened:** Agent tried to verify an infra-proven requirement (worker stack isolation) without test files. Got `TEST_FILES_REQUIRED` with `next: { mode: "none" }`. Had to independently discover `@manual` exists by reading `req verify --help`.
- **Root cause:** The error response had empty next steps. The `@manual` tag was documented only in the `--test-file` flag description of `req verify --help` — not surfaced at the moment of failure.
- **Fix:** `TEST_FILES_REQUIRED` now returns `next: { mode: "choose_one" }` with two options: (1) provide a test file, (2) guidance on adding `@manual` for infra-proven/inspection-verified behaviors. Help text also expanded with `@manual` usage scenarios.

### ~~Tag Registration Friction~~ (Fixed)

**`req add` rejects unknown Gherkin tags with `INVALID_GHERKIN_TAG`**
- **What happened:** `specctl req add` with `@conformance` and `@webhook` tags failed because these tags weren't in `.specs/specctl.yaml`'s `gherkin_tags` list. The error didn't guide the agent to the existing `config add-tag` command.
- **Fix:** INVALID_GHERKIN_TAG error now computes missing tags and returns `next` steps: `config add-tag <tag>` for each missing tag, then retry the original `req add`. No new commands needed — `config add-tag` already manages `gherkin_tags`.

### ~~Duplicate Requirement Registration~~ (Fixed)

**`req add` allows duplicate Feature titles within the same spec**
- **What happened:** `req add` allowed two requirements with identical Feature titles, creating duplicates that required manual withdrawal.
- **Fix:** `req add` now rejects with `REQUIREMENT_ALREADY_TRACKED` when a non-withdrawn requirement with the same Feature title already exists in the spec.

### ~~Drift Classifier Counts Closed Deltas~~ (Fixed)

**Closed deltas counted as "tracking" drift**
- **What happened:** `scope_drift.tracked_by` included closed delta IDs. Drift showed as `tracked` when it should be `drifted`, suggesting `rev_bump` instead of `delta add`.
- **Fix:** `deltaCoverageFiles` now filters both deferred and closed deltas. Only open and in-progress deltas count as tracking drift.

### ~~Changelog Never Records First-Batch Items~~ (Fixed)

**Rev bump omits items committed before the first revision**
- **What happened:** REQ-001–013 (from D-001) didn't appear in any changelog `reqs_added`; D-001 didn't appear in `deltas_closed`. The baseline comparison saw no diff because these items existed before the first rev bump.
- **Fix:** All 4 changelog fields (`deltas_closed`, `deltas_opened`, `reqs_added`, `reqs_verified`) now use changelog-aware computation — anything that exists but isn't recorded in any prior changelog entry gets included in the next rev bump.

### ~~Diff Suggests Wrong Next Action After Delta Close~~ (Fixed)

**Fully verified spec with closed deltas suggests semantic diff instead of rev bump**
- **What happened:** After fixing the drift classifier, a spec with all requirements verified and only closed deltas showed semantic diff options instead of suggesting `rev_bump`.
- **Fix:** Added `hasUnbumpedClosedDeltas` check in `buildSpecDiffNext` — when verified with no open deltas but unbumped closed deltas, suggests rev bump.

---

## Open — Agent Misuse Patterns

### `specctl config show` — nonexistent subcommand
- **What happened:** Agent guessed `specctl config show` without checking `specctl config --help` first.
- **Error:** `INVALID_INPUT — accepts 0 arg(s), received 1`
- **Root cause:** Agent assumed a `show` subcommand by analogy with other CLIs.
- **Mitigation:** Agents should always run `<command> --help` before guessing subcommands.
- **Possible fix:** Accept common aliases (`show`, `list`, `get`) as no-ops that redirect with a hint message.

---

## Open — Validation Blocking

### `charter create` blocked by invalid sibling spec
- **What happened:** `specctl charter create runtime` failed because an unrelated spec (`ouroboros/specctl.yaml`) had validation errors (`last_verified_at` missing, `checkpoint: null`).
- **Root cause:** Global validation on every write — any invalid tracking file anywhere in `.specs/` blocks all writes, even to unrelated charters.
- **Impact:** Agent had to manually patch unrelated YAML before it could create a new charter.
- **Possible fixes:**
  1. **Scoped validation:** Only validate the charter being written to, not the entire registry.
  2. **`specctl doctor` command:** Auto-repair common tracking file issues (missing fields, null checkpoints).

### Cannot `req verify` when existing test_files are missing
- **What happened:** After deleting `.feature` files, the tracking file referenced non-existent paths. `REQUIREMENT_TEST_FILE_MISSING` blocked ALL writes — including `req verify` (which would fix the references).
- **Root cause:** Global validation runs before any write. Dead test_file references make the tracking file invalid and no specctl command can repair it.
- **Workaround:** Hand-edit tracking YAML to remove dead paths. Violates specctl's "agents never hand-edit tracking YAML" rule.
- **Possible fixes:**
  1. **`req verify` should bypass test_file validation** for the requirement being re-verified.
  2. **`specctl doctor`:** Auto-detect and remove references to non-existent test files, downgrading verification to `unverified`.
  3. **`specctl hook`** should warn when staged deletions include tracked test files.

---

## Open — Verification Quality

### `req verify --test-file` accepts non-executable files
- **What happened:** Agent verified requirements against `.feature` files (Gherkin specs) instead of executable test files. specctl accepted them without warning.
- **Impact:** Requirements appeared "verified" with no actual test implementation.
- **Root cause:** specctl only checks that `--test-file` paths exist. It doesn't distinguish spec definitions from executable tests.
- **Possible fixes:**
  1. **Extension allowlist:** Only accept common test extensions (`_test.go`, `.spec.ts`, `.test.ts`, `.test.py`, etc.) and warn on `.feature`, `.md`, `.yaml`.
  2. **Separate flags:** `--spec-file` (informational) vs `--test-file` (verification proof).
