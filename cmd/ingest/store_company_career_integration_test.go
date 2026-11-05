//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/strelov1/freehire/internal/ingest/pipeline"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/job/job"
	"github.com/strelov1/freehire/internal/job/jobderive"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/testdb"
)

type companyCareerFeed struct {
	removed  bool
	deadline time.Time
}

func (companyCareerFeed) Provider() string { return "careerpage" }
func (s companyCareerFeed) Fetch(context.Context, sources.CompanyEntry) ([]sources.Job, error) {
	return []sources.Job{{ExternalID: "company-career-pilot", URL: "https://example.test/job/python-pilot", Title: "Python Backend Engineer", Company: "Career Pilot BD", Location: "Dhaka, Bangladesh", Description: "<p>Build Python and Django APIs with PostgreSQL.</p>", ApplicationDeadline: &s.deadline, Removed: s.removed}}, nil
}
func TestCompanyCareerIngestUpdatesDeadlineAndDeduplicates(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	store := newDBStore(pool, 1, nil, nil, defaultSeenPolicy())
	e := sources.CompanyEntry{Company: "Career Pilot BD", Provider: "careerpage", Board: "https://example.test/company-career-pilot"}
	deadline := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	crawl := func(feed companyCareerFeed) {
		t.Helper()
		runner := pipeline.Runner{Registry: map[string]sources.Source{"careerpage": feed}, Store: store}
		stats, err := runner.Run(ctx, []sources.CompanyEntry{e})
		if err != nil || stats.Total().Failed != 0 {
			t.Fatalf("crawl failed: %+v %v", stats, err)
		}
	}
	crawl(companyCareerFeed{deadline: deadline})
	externalID := e.Board + ":company-career-pilot"
	before, err := q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "careerpage", ExternalID: externalID})
	if err != nil {
		t.Fatal(err)
	}
	if !before.ApplicationDeadline.Valid || !before.ApplicationDeadline.Time.Equal(deadline) {
		t.Fatal("source deadline was not saved")
	}
	crawl(companyCareerFeed{deadline: deadline})
	revised := deadline.Add(24 * time.Hour)
	crawl(companyCareerFeed{deadline: revised})
	after, err := q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "careerpage", ExternalID: externalID})
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.PublicSlug != before.PublicSlug || !after.ApplicationDeadline.Time.Equal(revised) {
		t.Fatal("repeat crawl duplicated job or lost deadline-only edit")
	}
	if searchOutboxCount(t, pool, after.ID) != 1 {
		t.Fatal("ingested job was not queued for search")
	}
	crawl(companyCareerFeed{deadline: revised, removed: true})
	after, err = q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "careerpage", ExternalID: externalID})
	if err != nil || !after.ClosedAt.Valid {
		t.Fatal("explicit source removal did not close job")
	}
	// Other adapters that cannot state a deadline preserve a previously known one.
	unknown, err := job.New(job.Draft{Input: jobderive.Input{Source: "careerpage", ExternalID: externalID, Title: before.Title, Company: before.Company, Location: before.Location, Description: before.Description}, URL: before.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, unknown); err != nil {
		t.Fatal(err)
	}
	after, err = q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "careerpage", ExternalID: externalID})
	if err != nil || !after.ApplicationDeadline.Time.Equal(revised) {
		t.Fatal("unknown deadline erased known deadline")
	}
}
