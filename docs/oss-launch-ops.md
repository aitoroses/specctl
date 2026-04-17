# OSS Launch Ops

This file records the repository settings required for the public launch bar.

## Required settings

- Branch protection on `main`
- Pull requests required for merge
- At least 1 approving human review required
- Required status checks:
  - `go`
  - `dashboard`
- Force pushes disabled
- Branch deletion disabled
- Release/tag publishing restricted to the human maintainer or explicitly named human approvers

## Evidence

- Date: 2026-04-17
- Approver GitHub login: `aitoroses`
- CI run evidence: https://github.com/aitoroses/specctl/actions/runs/24566285973
- Branch protection API evidence: `gh api repos/aitoroses/specctl/branches/main/protection`
- Required checks: `go`, `dashboard`
- Pull-request merge required: yes
- Required human approvals: 1
- Force pushes disabled: yes
- Branch deletion disabled: yes
- Conversation resolution required: yes
- Linear history required: yes
- Direct collaborators evidence: `gh api 'repos/aitoroses/specctl/collaborators?affiliation=direct'` shows only `aitoroses`
- Release/tag restriction evidence: operationally restricted to the human maintainer account controlling releases for this repository, and the current direct collaborator/admin set contains only `aitoroses`

## Approval statement

I approve the configured merge/release boundary for the public launch bar of `specctl`.
