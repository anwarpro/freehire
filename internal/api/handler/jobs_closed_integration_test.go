//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/job/jobview"
	"github.com/strelov1/freehire/internal/platform/db"
)

func TestClosedJobBrowsing(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	closedAt := time.Now().UTC().Truncate(time.Microsecond)
	insert := func(slug, source string, skills, countries []string, closed, private bool) int64 {
		t.Helper()
		var at *time.Time
		if closed {
			at = &closedAt
		}
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO jobs (source,external_id,url,title,public_slug,skills,countries,closed_at,is_private) VALUES ($1,$2,'https://example.test/original','Engineer',$2,$3,$4,$5,$6) RETURNING id`, source, slug, skills, countries, at, private).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	canonical := insert("closed-z", "bdjobs", []string{"python", "react"}, []string{"bd"}, true, false)
	insert("closed-y", "bdjobs", []string{"python"}, []string{"bd"}, true, false)
	insert("closed-x", "greenhouse", []string{"go"}, []string{"us"}, true, false)
	insert("private-z", "bdjobs", []string{"python"}, []string{"bd"}, true, true)
	insert("active-z", "bdjobs", []string{"python"}, []string{"bd"}, false, false)
	duplicate := insert("duplicate-z", "bdjobs", []string{"python"}, []string{"bd"}, true, false)
	if _, err := pool.Exec(ctx, `UPDATE jobs SET duplicate_of_role=$1 WHERE id=$2`, canonical, duplicate); err != nil {
		t.Fatal(err)
	}
	h := &jobsHandlers{queries: db.New(pool)}
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	app.Get("/api/v1/jobs/closed", h.ClosedJobs)
	type response struct {
		Data []jobview.Job `json:"data"`
		Meta struct {
			More bool   `json:"has_more"`
			Next string `json:"next_cursor"`
		} `json:"meta"`
	}
	read := func(query string) response {
		t.Helper()
		res, err := app.Test(httptest.NewRequest("GET", "/api/v1/jobs/closed?"+query, nil))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("query %s returned %d", query, res.StatusCode)
		}
		var out response
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := read("limit=1&source=bdjobs&skills=python&countries=bd")
	if len(first.Data) != 1 || first.Data[0].PublicSlug != "closed-z" || !first.Meta.More || first.Meta.Next == "" {
		t.Fatalf("unexpected first page: %+v", first)
	}
	original, parseErr := url.Parse(first.Data[0].URL)
	if first.Data[0].ClosedAt == nil || parseErr != nil || original.Host != "example.test" || original.Path != "/original" {
		t.Fatal("archive lost closed state or original link")
	}
	next := read("limit=1&source=bdjobs&skills=python&countries=bd&before=" + url.QueryEscape(first.Meta.Next))
	if len(next.Data) != 1 || next.Data[0].PublicSlug != "closed-y" || next.Meta.More || next.Meta.Next != "" {
		t.Fatalf("unexpected second page: %+v", next)
	}
	all := read("limit=20")
	if len(all.Data) != 3 {
		t.Fatalf("private, active or duplicate job leaked: %+v", all.Data)
	}
	both := read("skills=python,react&skills_mode=and")
	if len(both.Data) != 1 || both.Data[0].PublicSlug != "closed-z" {
		t.Fatalf("all-skills filter failed: %+v", both.Data)
	}
	excluded := read("source_exclude=greenhouse&skills_exclude=react&countries_exclude=us")
	if len(excluded.Data) != 1 || excluded.Data[0].PublicSlug != "closed-y" {
		t.Fatalf("exclusions failed: %+v", excluded.Data)
	}
	empty := read("countries=jp")
	if len(empty.Data) != 0 || empty.Meta.More {
		t.Fatal("empty result is not empty")
	}
	raw, _ := json.Marshal(all.Data)
	if strings.Contains(string(raw), `"id":`) || strings.Contains(string(raw), "private-z") {
		t.Fatal("internal identity or private data leaked")
	}
}
