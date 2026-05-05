package daemon

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/carsteneu/yesmem/internal/codescan"
	"github.com/carsteneu/yesmem/internal/storage"
	"github.com/carsteneu/yesmem/internal/wikirender"
)

const wikiTickInterval = 5 * time.Minute

func startWikiTicker(ctx context.Context, store *storage.Store) {
	go func() {
		ticker := time.NewTicker(wikiTickInterval)
		defer ticker.Stop()
		home, _ := os.UserHomeDir()
		outRoot := filepath.Join(home, ".claude", "yesmem", "wiki")
		runWikiOnce(ctx, store, outRoot)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runWikiOnce(ctx, store, outRoot)
			}
		}
	}()
}

func runWikiOnce(ctx context.Context, store *storage.Store, outRoot string) {
	projects, err := activeProjects(ctx, store)
	if err != nil {
		log.Printf("wiki-tick: list projects: %v", err)
		return
	}

	// Build a fresh code graph from CBM if available — captures imports, calls, etc.
	// Uses own scanner to avoid the handler's cache (which may be stale).
	cbmBin := codescan.FindCBMBinary()
	for _, p := range projects {
		dir := store.ResolveProjectPath(p)
		out := filepath.Join(outRoot, p)

		var graph *codescan.CodeGraph
		if cbmBin != "" && dir != "" {
			scanner := codescan.NewCBMScanner()
			if sr, err := scanner.Scan(dir); err == nil {
				graph = codescan.BuildCodeGraph(sr)
				// Cache scan result so next tick skips CBM subprocess.
				if raw, err := json.Marshal(sr); err == nil {
					store.SaveProjectScan(&storage.ProjectScanRow{
						Project: p, ScanJSON: string(raw),
					})
				}
			}
		}

		r, err := wikirender.Render(ctx, wikirender.RenderConfig{
			Project:   p,
			OutputDir: out,
			Store:     store,
			CodeGraph: graph,
			Quiet:     true,
		})
		if err != nil {
			log.Printf("wiki-tick: render %s: %v", p, err)
			continue
		}
		log.Printf("wiki-tick: %s learnings=%d topics=%d files=%d sessions=%d in %dms",
			r.Project, r.Learnings, r.Topics, r.Files, r.Sessions, r.DurationMs)
	}
}

func activeProjects(ctx context.Context, store *storage.Store) ([]string, error) {
	rows, err := store.DB().QueryContext(ctx, `
		SELECT DISTINCT project FROM learnings
		WHERE project IS NOT NULL AND project != '' AND superseded_by IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
