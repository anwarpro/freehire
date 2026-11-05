//go:build integration

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/strelov1/freehire/internal/ingest/linkimport"
	"github.com/strelov1/freehire/internal/ingest/pipeline"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/testdb"
	"golang.org/x/net/html"
)

const linkedinStoreHTML = `<h2 class="top-card-layout__title">Python Engineer</h2><a class="topcard__org-name-link">LinkedIn Integration BD</a><span class="topcard__flavor--bullet">Dhaka, Bangladesh</span><div class="description__text"><div class="show-more-less-html__markup"><p>Build Python Django APIs and PostgreSQL services.</p></div></div>`

type linkedinStoreFixture struct {
	body string
	fail bool
}

func (f *linkedinStoreFixture) Provider() string { return "linkedin" }
func (f *linkedinStoreFixture) Fetch(context.Context, sources.CompanyEntry) ([]sources.Job, error) {
	if f.fail {
		return nil, fmt.Errorf("access refused")
	}
	node, _ := html.Parse(strings.NewReader(f.body))
	return []sources.Job{sources.ParseLinkedInJob(node, "910000002")}, nil
}
func (f *linkedinStoreFixture) GetHTML(context.Context, string) (*html.Node, error) {
	return html.Parse(strings.NewReader(f.body))
}
func (f *linkedinStoreFixture) GetHTMLResolved(ctx context.Context, raw string) (*html.Node, string, error) {
	n, err := f.GetHTML(ctx, raw)
	return n, raw, err
}
func (*linkedinStoreFixture) GetJSON(context.Context, string, any) error {
	return fmt.Errorf("unexpected JSON request")
}

func TestLinkedInIntakeDeduplicatesAcrossLinksAndCrawlAndPreservesLifecycle(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	feed := &linkedinStoreFixture{body: linkedinStoreHTML}
	im := linkimport.New(pool, q, nil, feed, nil, nil)
	imported, ok, err := im.Import(ctx, "https://bd.linkedin.com/jobs/view/python-engineer-910000002?trk=shared", linkimport.Board{})
	if err != nil || !ok {
		t.Fatalf("link import: %v %v", ok, err)
	}
	read := func() db.Job {
		t.Helper()
		j, err := q.GetJobBySourceExternalID(ctx, db.GetJobBySourceExternalIDParams{Source: "linkedin", ExternalID: ":910000002"})
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	before := read()
	if before.PublicSlug != imported.PublicSlug || before.URL != "https://www.linkedin.com/jobs/view/910000002/" {
		t.Fatal("canonical link provenance lost")
	}
	runner := pipeline.Runner{Registry: map[string]sources.Source{"linkedin": feed}, Store: newDBStore(pool, 1, nil, nil, defaultSeenPolicy())}
	crawl := func(failed bool) {
		t.Helper()
		stats, err := runner.Run(ctx, []sources.CompanyEntry{{Provider: "linkedin", Company: "LinkedIn"}})
		if err != nil || (stats.Total().Failed > 0) != failed {
			t.Fatalf("crawl %+v %v", stats, err)
		}
	}
	crawl(false)
	crawl(false)
	if after := read(); after.ID != before.ID || after.PublicSlug != before.PublicSlug {
		t.Fatal("crawl duplicated imported job")
	}
	feed.body = strings.ReplaceAll(linkedinStoreHTML, "Build Python Django APIs and PostgreSQL services.", "Build Python Django APIs, PostgreSQL services and machine learning pipelines.")
	crawl(false)
	if !strings.Contains(read().Description, "machine learning pipelines") {
		t.Fatal("source edit not refreshed")
	}
	feed.fail = true
	crawl(true)
	if read().ClosedAt.Valid {
		t.Fatal("failed crawl closed job")
	}
	feed.fail = false
	feed.body = `<h1>Sign in to continue</h1>`
	crawl(false)
	if read().ClosedAt.Valid {
		t.Fatal("unreadable detail closed job")
	}
	feed.body = `<span class="closed-job__flavor">No longer accepting applications</span>`
	crawl(false)
	if !read().ClosedAt.Valid {
		t.Fatal("explicit removal did not close job")
	}
	feed.body = linkedinStoreHTML
	crawl(false)
	if read().ClosedAt.Valid {
		t.Fatal("relisted job did not reopen")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE source='linkedin'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate count %d %v", count, err)
	}
	if searchOutboxCount(t, pool, before.ID) == 0 {
		t.Fatal("job missing public search outbox")
	}
}
