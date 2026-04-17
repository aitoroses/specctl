---
spec: session-lifecycle
charter: runtime
---
# Session Lifecycle

## Requirement: Compensation stage 4 failure cleanup

```gherkin requirement
@runtime @e2e
Feature: Compensation stage 4 failure cleanup
```

### Scenarios

```gherkin scenario
Scenario: Cleanup runs after stage 4 failure
  Given stage 4 fails during compensation
  When recovery completes
  Then cleanup steps run in documented order
```
