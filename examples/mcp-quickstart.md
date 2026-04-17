# MCP quickstart

Start the MCP server:

```bash
specctl mcp
```

The server exposes `specctl_*` tools over stdio.

Expected smoke check:

- initialize MCP client
- list tools
- confirm tools such as:
  - `specctl_context`
  - `specctl_diff`
  - `specctl_requirement_verify`
