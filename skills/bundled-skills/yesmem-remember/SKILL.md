---
name: yesmem-remember
description: Use after ANY decision, correction, gotcha, or discovery worth preserving. Also on task completion — resolve open tasks via resolve_by_text() after commits, "fertig", or confirmed fixes. Trigger on "remember this"/"merk dir das", confirmed approaches, debugging surprises, and relate_learnings() for connecting insights.
---

# Saving Knowledge

Store learnings with structured metadata for better retrieval.

## Workflow
1. Identify what to save (decision, gotcha, pattern, preference, teaching)
2. Call `remember()` with rich metadata
3. For completed tasks: `resolve_by_text(text)` or `resolve(learning_id, reason)`
4. For connected insights: `relate_learnings(id_a, id_b, relation_type)`

## remember() Parameters

| Parameter | Purpose | Example |
|-----------|---------|---------|
| `text` | What to remember | "CGO is disabled in yesmem Makefile" |
| `category` | gotcha, decision, pattern, preference, explicit_teaching, strategic | "decision" |
| `entities` | Files, systems, people | ["Makefile", "CGO"] |
| `actions` | Commands, operations | ["go build", "make deploy"] |
| `trigger` | When to surface this | "When discussing CGO or cross-compilation" |
| `context` | Why this matters | "Breaks tree-sitter integration" |
| `anticipated_queries` | 3-5 search phrases | ["CGO disabled", "cross compile yesmem"] |
| `source` | user_stated, agreed_upon, claude_suggested | "agreed_upon" |
| `project` | Project scope | "yesmem" |
| `supersedes` | ID of learning this replaces | 12345 |
| `force` | Store even when the text is a near-duplicate | true |

### What the answer means

`remember()` never drops a text silently. Read the answer before moving on:

| Answer | Meaning |
|--------|---------|
| `✓ Learning #N saved` | stored as a new row |
| `NOT stored … near-duplicate of #N` (`stored=false`) | nothing was written; the response names the entry you collided with |
| `Already known as #N (identical text)` | the same text is already stored; its `match_count` was bumped |

If the text carries new information that the containment similarity cannot see, repeat the call with `force=true`. If it **corrects** an existing entry, just write it: an opposite-polarity text (e.g. "… ist nicht blind" against "… ist blind") is stored as a correction, numbered against the entry it contradicts, and that entry is left untouched. Use `supersedes=<id>` only when you want the old entry replaced.

## relate_learnings()

Connect two learnings with semantic edges:
- `supports` — one learning confirms another
- `contradicts` — learnings conflict
- `depends_on` — one requires the other
- `relates_to` — general connection

## Tips
- `anticipated_queries`: vary between short keywords and full questions
- `source` trust hierarchy: user_stated > agreed_upon > claude_suggested
- Use `supersedes` when updating outdated knowledge — don't create duplicates
- `resolve_by_text` finds matching open tasks by text search
