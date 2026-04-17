# Security Policy

## Supported versions

Security fixes are provided on the default branch and, when releases exist, on the latest stable release line judged practical by the maintainers.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead:

1. Email the maintainer directly or use GitHub private vulnerability reporting if enabled.
2. Include:
   - affected version / commit
   - reproduction steps
   - impact assessment
   - any suggested mitigation

## Response expectations

- Initial acknowledgement: best effort
- Triage: best effort
- Disclosure timing: coordinated with the maintainer after remediation is understood

Because `specctl` is an **agent-facing tool**, please clearly state whether the issue affects:

- the CLI surface
- the MCP surface
- the packaged skill
- release/install workflow

## Public disclosure

Public disclosure should wait until:

- the issue is understood
- a fix or mitigation exists
- the maintainer approves publication
