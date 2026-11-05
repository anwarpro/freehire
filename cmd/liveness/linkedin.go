package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/job/liveness"
	"github.com/strelov1/freehire/internal/platform/db"
	"golang.org/x/net/html"
)

// LinkedIn's bounded discovery cannot re-reach every old posting. Check a
// rotating, paced sample after 48h unseen; only explicit removal earns a strike.
func applyLinkedInVerdicts(ctx context.Context, client *http.Client, q *db.Queries, rows []candidate, s *stats) {
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	if len(rows) > 100 {
		rows = rows[:100]
	}
	for i, c := range rows {
		if i > 0 {
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		verdict, reason := checkLinkedInLiveAt(ctx, client, c.externalID, sources.LinkedInDetailURL)
		s.apply(ctx, q, c, verdict, reason)
		if verdict == liveness.Uncertain {
			log.Printf("liveness: linkedin could not verify detail; deferring remaining %d probes", len(rows)-i-1)
			return
		}
	}
}

func checkLinkedInLiveAt(ctx context.Context, client *http.Client, externalID, endpoint string) (liveness.Verdict, string) {
	id := strings.TrimPrefix(externalID, ":")
	if native := sources.LinkedInJobID(id); native != "" {
		id = native
	}
	if id == "" || sources.LinkedInJobID("https://www.linkedin.com/jobs/view/"+id+"/") != id {
		return liveness.Uncertain, ""
	}
	requested := endpoint + id
	status, final, body, err := liveness.Fetch(ctx, client, requested)
	if err != nil || final != requested {
		return liveness.Uncertain, ""
	}
	if status == 404 || status == 410 {
		return liveness.Expired, "linkedin_job_gone"
	}
	if status != 200 {
		return liveness.Uncertain, ""
	}
	node, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return liveness.Uncertain, ""
	}
	j := sources.ParseLinkedInJob(node, id)
	if j.Removed {
		return liveness.Expired, "linkedin_no_longer_accepting"
	}
	if j.Unreadable {
		return liveness.Uncertain, ""
	}
	return liveness.Live, ""
}
