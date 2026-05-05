package wikirender

const learningTpl = `---
id: {{.ID}}
trust: {{.Source}}
trust_score: {{trustScore .Source false}}
origin_tool: {{.OriginTool}}
session: {{.SessionID}}
category: {{.Category}}
created: {{.CreatedAt}}
---

# {{badge .Source false}} Learning {{.ID}}{{if .QuarantinedAt}}

> ⚠️  **QUARANTINED** since {{.QuarantinedAt}} — content excluded from active recall.{{end}}
{{if .Supersedes}}
← Evolved from [#{{.Supersedes}}]({{.Supersedes}}.md){{if .SupersedeReason}} — _{{.SupersedeReason}}_{{end}}

{{end}}
**Source:** {{.Source}} · **Category:** {{.Category}} · **Created:** {{.CreatedAt}} · **Uses:** {{.UseCount}}{{if .QuarantinedAt}} · **QUARANTINED:** {{.QuarantinedAt}}{{end}}

{{.Content}}

{{if .TriggerRule}}**Trigger:** {{.TriggerRule}}

{{end}}{{if .Context}}**Context:** {{.Context}}

{{end}}{{if .TaskType}}**Task:** {{.TaskType}}

{{end}}
{{if or .SessionID .ModelUsed .OriginTool .AgentRole .Domain .DialogID}}## Provenance

{{if .SessionID}}- **Session:** [{{.SessionID}}](../sessions/{{.SessionID}}.md){{if and (gt .SourceMsgFrom -1) (gt .SourceMsgTo -1)}} (msg {{.SourceMsgFrom}}–{{.SourceMsgTo}}){{end}}
{{end}}{{if .ModelUsed}}- **Model:** {{.ModelUsed}}
{{end}}{{if .OriginTool}}- **Origin tool:** {{.OriginTool}}
{{end}}{{if .AgentRole}}- **Agent role:** {{.AgentRole}}
{{end}}{{if .Domain}}- **Domain:** {{.Domain}}
{{end}}{{if .DialogID}}- **Dialog:** {{.DialogID}}
{{end}}
{{end}}
## Trust

- **Confidence:** {{printf "%.2f" .Confidence}}
- **Importance:** {{printf "%.2f" .Importance}}
- **Stability:** {{printf "%.2f" .Stability}}
{{if .EmbeddingStatus}}- **Embedding:** {{.EmbeddingStatus}}
{{end}}
## Engagement

- **Hits:** {{.HitCount}}
- **Injects:** {{.InjectCount}}
- **Saves:** {{.SaveCount}}
{{if .LastHitAt}}- **Last hit:** {{.LastHitAt}}
{{end}}- **Turns at creation:** {{.TurnsAtCreation}}
{{if .Entities}}
## Entities

{{range .Entities}}- [{{.}}](../topics/{{slugify .}}.md)
{{end}}{{end}}{{if .Actions}}
## Actions

{{range .Actions}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .Keywords}}
## Keywords

{{range .Keywords}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .Related}}
## Verknüpfung

{{range .Related}}- {{badge .Source false}} [#{{.ID}}]({{.ID}}.md) — overlap {{.Overlap}} — {{.Snippet}}
{{end}}{{end}}
## Audit

Run ` + "`hybrid_search(\"{{top3 .Entities}}\")`" + ` to find related learnings.
`

const topicTpl = `# Topic: {{.Name}}

**Linked learnings:** {{len .Learnings}}
**Span:** {{spanFirst .Learnings}} → {{spanLast .Learnings}}

## Trust

{{trustMix .Learnings}}
## Categories

{{categoryMix .Learnings}}{{if .CoTopics}}
## Co-occurring Topics

{{range .CoTopics}}- [{{.Name}}]({{slugify .Name}}.md) — shared {{.Shared}}
{{end}}{{end}}
## Learnings

{{range .Learnings}}- {{badge .Source false}} [Learning {{.ID}}](../learnings/{{.ID}}.md) ({{.Category}}): {{snippet .Content 120}}
{{end}}`

const fileTpl = `# File: {{.Path}}

**Directory:** ` + "`{{.Directory}}`" + `
**Sessions touched:** {{.SessionCount}} — last {{daysAgo .LastTouched}}
**Operations:** {{.OperationTypes}}
{{if .Code}}**Language:** {{.Code.Language}} · **LOC:** {{.Code.LOC}}{{if .Code.IsTest}} · **TEST FILE**{{end}}{{if gt .Code.TestCount 0}} · Test files: {{.Code.TestCount}}{{end}}
{{end}}
{{if .Code}}{{if .Code.Signatures}}
## Symbols ({{len .Code.Signatures}})

{{range .Code.Signatures}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .Code.Imports}}
## Imports ({{len .Code.Imports}})

{{range .Code.Imports}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{end}}{{if .Package}}**Package:** [` + "`{{.Package}}`" + `](../packages/{{fileSlug .Package}}.md)
{{if .Imports}}
## Imports ({{len .Imports}})

{{range .Imports}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .ImportedBy}}
## Imported by ({{len .ImportedBy}})

{{range .ImportedBy}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{end}}{{if .CoEdited}}
## Often Co-edited

{{range .CoEdited}}- ` + "`{{.Path}}`" + ` ({{.Count}} times)
{{end}}{{end}}{{if .Learnings}}
## Learnings about this file ({{len .Learnings}})

{{range .Learnings}}- {{badge .Source false}} [#{{.ID}}](../learnings/{{.ID}}.md) ({{.Category}}): {{snippet .Content 600}}
{{end}}{{end}}{{if .Sessions}}
## Recent Sessions ({{len .Sessions}})

{{range .Sessions}}- [{{.ID}}](../sessions/{{.ID}}.md) — {{.StartedAt}} — {{.Messages}} msgs
{{end}}{{end}}`

const packagesIndexTpl = `# Packages — {{.Project}}

{{range .Packages}}- [{{.Name}}](packages/{{fileSlug .Name}}.md) · {{.FileCount}} files{{if .Language}} · {{.Language}}{{end}}{{if gt .Gotchas 0}} · {{.Gotchas}} gotchas{{end}}{{if gt .TODOs 0}} · {{.TODOs}} TODOs{{end}}
{{end}}`

const sessionTpl = `# Session {{.ShortID}}

Full id: ` + "`{{.ID}}`" + `

Learnings created: {{len .Learnings}}
{{range .Learnings}}
## Learning {{.ID}}

**Category:** {{.Category}} · **Source:** _{{.Source}}_

{{.Content}}
{{end}}`

const readmeTpl = `# Wiki: {{.Project}}

Generated: {{.BuiltAt}}

## Browse

- [learnings/](learnings/) — {{.LearningsCount}} files
- [topics/](topics/) — {{.TopicsCount}} entities
- [files/](files/) — {{.FilesCount}} source files
- [packages/](packages/) — {{.PackagesCount}} packages
- [sessions/](sessions/) — {{.SessionsCount}} sessions
- [health.md](health.md)
{{if .RecentSessions}}
## Recent sessions

{{range .RecentSessions}}- [{{.ShortID}}](sessions/{{.ShortID}}.md)
{{end}}{{end}}`

const indexTpl = `# Files — {{.Project}}

{{range .Dirs}}## {{.Name}} ({{len .Files}})

{{range .Files}}- [{{base .Path}}](files/{{fileSlug .Path}}.md) · {{.SessionCount}} sessions · last {{daysAgo .LastTouched}}
{{end}}{{end}}`

const learningsIndexTpl = `# Learnings — {{.Project}}

{{range .Categories}}- [{{.Name}} ({{len .Learnings}})](#{{slugify .Name}})
{{end}}
{{range .Categories}}## {{.Name}} ({{len .Learnings}})

{{range .Learnings}}- {{badge .Source false}} [#{{.ID}}](learnings/{{.ID}}.md): {{snippet .Content 100}}
{{end}}{{end}}`

const healthTpl = `# Health: {{.Project}}

Built: {{.BuiltAt}}

## Counts

- Active learnings: {{.LearningsCount}}{{if gt .QuarantinedCount 0}}
- Quarantined: {{.QuarantinedCount}}{{end}}
- Entity links: 0
- Files tracked: {{.FilesCount}}
- Open contradictions: {{len .Contradictions}}
{{if .Contradictions}}
## Open contradictions

{{range .Contradictions}}- {{.ID}}: {{.Description}} — IDs: {{.LearningIDs}}
{{end}}{{end}}`

const packageTpl = `# Package: {{.Name}}

{{if .Intent}}> {{.Intent}}
{{end}}
**Files:** {{.FileCount}}{{if .Language}} · **Language:** {{.Language}}{{end}}{{if gt .TestCount 0}} · Test files: {{.TestCount}}{{end}}{{if .LastEdited}} · Last edited: {{.LastEdited}}{{end}}

{{if .Files}}
## Files ({{len .Files}})

{{range .Files}}- [{{base .Path}}](files/{{fileSlug .Path}}.md){{if gt .SessionCount 0}} · {{.SessionCount}} sessions{{end}}
{{end}}{{end}}{{if .Symbols}}
## Symbols ({{len .Symbols}})

{{range .Symbols}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .Imports}}
## Imports ({{len .Imports}})

{{range .Imports}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .ImportedBy}}
## Imported by ({{len .ImportedBy}})

{{range .ImportedBy}}- ` + "`{{.}}`" + `
{{end}}{{end}}{{if .CoEdited}}
## Co-edited with

{{range .CoEdited}}- ` + "`{{.Path}}`" + ` ({{.Count}} times)
{{end}}{{end}}{{if .Learnings}}
## Learnings ({{len .Learnings}})

{{range .Learnings}}- {{badge .Source false}} [#{{.ID}}](../learnings/{{.ID}}.md) ({{.Category}}): {{snippet .Content 120}}
{{end}}{{end}}
## Health

- Gotchas: {{.Gotchas}} · TODOs: {{.TODOs}}
{{if .Sessions}}
## Recent Sessions ({{len .Sessions}})

{{range .Sessions}}- [{{.ID}}](../sessions/{{.ID}}.md) — {{.StartedAt}} — {{.Messages}} msgs
{{end}}{{end}}`
