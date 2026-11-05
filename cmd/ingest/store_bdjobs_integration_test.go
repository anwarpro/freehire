//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/strelov1/freehire/internal/ingest/pipeline"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/testdb"
)

type bdjobsHTTPFixture struct {
	deadline string
	closed   bool
	fail     bool
}

func (f *bdjobsHTTPFixture) GetJSON(_ context.Context, u string, out any) error {
	if f.fail {
		return fmt.Errorf("listing unavailable")
	}
	var body string
	if strings.Contains(u, "GetJobSearch?") {
		body = fmt.Sprintf(`{"statuscode":"1","data":[{"Jobid":"910000001","jobTitle":"Python Engineer","companyName":"BDJobs Integration Pilot","publishDate":"%s","deadlineDB":"%sT00:00:00Z"}],"common":{"total_records_found":1,"totalpages":1}}`, time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339), f.deadline)
	} else {
		closed := 0
		if f.closed {
			closed = 1
		}
		body = fmt.Sprintf(`{"statuscode":"0","data":[{"JobId":"910000001","JobFound":"True","error":"0","JobTitle":"Python Engineer","CompnayName":"BDJobs Integration Pilot","JobDescription":"<p>Build Python and Django APIs with PostgreSQL.</p>","JobLocation":"Dhaka, Bangladesh","JobNature":"Full Time","JobWorkPlace":"Work at office","DeadlineDB":"%s","Closed":%d}]}`, f.deadline, closed)
	}
	return json.Unmarshal([]byte(body), out)
}

func TestBDJobsCollectorPersistsDeduplicatesUpdatesAndCloses(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	store := newDBStore(pool, 1, nil, nil, defaultSeenPolicy())
	feed := &bdjobsHTTPFixture{deadline: time.Now().Add(72 * time.Hour).UTC().Format("2006-01-02")}
	entry := sources.CompanyEntry{Provider: "bdjobs", Company: "BDJobs"}
	runner := pipeline.Runner{Registry: map[string]sources.Source{"bdjobs": sources.NewBDJobs(feed)}, Store: store}
	crawl := func(wantFailure bool) {
		t.Helper()
		stats, err := runner.Run(ctx, []sources.CompanyEntry{entry})
		if err != nil {
			t.Fatal(err)
		}
		if (stats.Total().Failed > 0) != wantFailure {
			t.Fatalf("unexpected crawl status: %+v", stats)
		}
	}
	read := func() db.Job {
		t.Helper()
		j, err := q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "bdjobs", ExternalID: ":910000001"})
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	crawl(false)
	before := read()
	if before.URL != "https://bdjobs.com/h/details/910000001" || !before.ApplicationDeadline.Valid || !before.IsTech.Valid || !before.IsTech.Bool || before.Company != "BDJobs Integration Pilot" || !before.PostedAt.Valid {
		t.Fatalf("lost source data: %+v", before)
	}
	if searchOutboxCount(t, pool, before.ID) != 1 {
		t.Fatal("not queued for public search")
	}
	crawl(false)
	after := read()
	if before.ID != after.ID || before.PublicSlug != after.PublicSlug {
		t.Fatal("repeat import changed stable identity")
	}
	feed.deadline = time.Now().Add(96 * time.Hour).UTC().Format("2006-01-02")
	crawl(false)
	after = read()
	if !after.ApplicationDeadline.Time.Equal(before.ApplicationDeadline.Time.Add(24 * time.Hour)) {
		t.Fatal("deadline-only source edit was not persisted")
	}
	feed.fail = true
	crawl(true)
	if read().ClosedAt.Valid {
		t.Fatal("failed listing closed an existing job")
	}
	feed.fail = false
	feed.closed = true
	crawl(false)
	if !read().ClosedAt.Valid {
		t.Fatal("source closure was not persisted")
	}
}
