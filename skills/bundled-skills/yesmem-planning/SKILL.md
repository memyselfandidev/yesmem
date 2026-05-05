---
name: yesmem-planning
description: Use when starting iterative work that needs to survive context-collapse, like work spanning more than 5 tool cycles, exploring more than 1 hypothesis, touching multiple files or worktrees, debug spirals, side-quests parallel to a main thread, or when prompted by [Plan Checkpoint]. Plans are thread-scoped (parallel sessions don't conflict) and re-injected on every turn. They are the only context-loss-proof anchor for the active task. Activate via set_plan(), update via update_plan() at each pivot.
---

# Plan Management

Track implementation plans with automatic checkpoint reminders.

## Workflow
1. `set_plan(plan)` — activate a plan (triggers checkpoints every ~20k tokens)
2. `update_plan(completed, add, remove)` — mark progress incrementally
3. `get_plan()` — check current plan state
4. `complete_plan()` — mark plan as done, stops checkpoints

## set_plan Format
Free text or structured list with markers:
```
- [x] Task 1 (done)
- [>] Task 2 (in progress)
- [ ] Task 3 (pending)
```

## update_plan Parameters

| Parameter | Purpose | Example |
|-----------|---------|---------|
| `completed` | Items to mark done (substring match) | ["Task 1", "schema migration"] |
| `add` | New items to append | ["Fix edge case in handler"] |
| `remove` | Items to remove (substring match) | ["Cancelled feature"] |
| `plan` | Replace entire plan | "New plan content..." |

## Tips
- Plan checkpoints inject automatically every ~20k tokens when a plan is active
- Reading a plan file triggers a nudge to call `set_plan()`
- `scope="persistent"` survives session end
- Call `complete_plan()` when ALL items are done — stops checkpoint injection
