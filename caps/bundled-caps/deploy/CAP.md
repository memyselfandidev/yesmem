---
name: deploy
description: "Run `make deploy` in the current yesmem worktree — builds binary, atomic-replaces ~/.local/bin/yesmem, restarts daemon+proxy services. Returns {version: \"vX.Y.Z-N-gHASH\", output} on success, {error, output} on fail. Optional `dir` arg overrides cwd."
version: 91
tags: [yesmem, build, deploy, local]
scope: user
tested: true
auto_active: true
---

## Purpose

Run `make deploy` in the current yesmem worktree — builds binary, atomic-replaces ~/.local/bin/yesmem, restarts daemon+proxy services. Returns {version: "vX.Y.Z-N-gHASH", output} on success, {error, output} on fail. Optional `dir` arg overrides cwd.

## Scripts

### deploy
kind: tool

```javascript
async ({ dir } = {}) => {
    const cmd = dir ? `cd ${shQuote(dir)} && make deploy 2>&1` : "make deploy 2>&1";
    const output = await sh(cmd, 20000);
    const m = output.match(/deployed\s+(v\S+)\s+→/);
    if (!m)
      return { error: 'deploy output missing "deployed vX.Y.Z" line', output };
    return { version: m[1], output };
  }
```
