// Command company-careers verifies the curated shortlist and optionally enables
// supported sources through the same service as the moderator UI.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"github.com/strelov1/freehire/internal/ingest/boardcatalog"
	"github.com/strelov1/freehire/internal/ingest/companycareer"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/worker"
)

func main() { worker.Main(run) }
func run() int {
	apply := flag.Bool("apply", false, "verify and enable the supported shortlist in Postgres")
	flag.Parse()
	ctx := context.Background()
	var repo boardcatalog.Repository
	if *apply {
		boot, _, pool, cleanup, err := worker.Bootstrap(ctx)
		if err != nil {
			log.Print(err)
			return 1
		}
		defer cleanup()
		ctx = boot
		repo = boardcatalog.NewQueriesRepository(db.New(pool))
	}
	svc := companycareer.New(repo)
	failed := false
	for _, seed := range companycareer.Shortlist() {
		if seed.Fallback != "" {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"company": seed.Company, "status": "manual_fallback", "reason": seed.Fallback})
			continue
		}
		input := companycareer.Input{Company: seed.Company, URL: seed.URL, Config: seed.Config}
		one, cancel := context.WithTimeout(ctx, 90*time.Second)
		var p companycareer.Preview
		var err error
		if *apply {
			_, p, err = svc.Enable(one, input)
		} else {
			p, err = svc.Preview(one, input)
		}
		cancel()
		status := "verified"
		if errors.Is(err, boardcatalog.ErrDuplicateBoard) {
			status = "already_enabled"
			err = nil
		}
		out := map[string]any{"company": seed.Company, "status": status, "active": p.Active, "closed": p.Closed, "samples": p.Samples}
		if err != nil {
			failed = true
			out["status"] = "verification_failed"
			out["error"] = err.Error()
		}
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			log.Print(err)
			return 1
		}
	}
	if failed {
		return 1
	}
	return 0
}
