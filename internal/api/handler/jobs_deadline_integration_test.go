//go:build integration

package handler

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"testing"
	"time"

	"github.com/strelov1/freehire/internal/ai/enrich"
	"github.com/strelov1/freehire/internal/ingest/moderation"
	"github.com/strelov1/freehire/internal/platform/db"
)

func TestModeratorDeadlineLifecycle(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	q := db.New(pool)
	var actor int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, role) VALUES ('deadline@example.test','moderator') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	svc := moderation.New(moderation.NewQueriesRepository(q, pool, enrich.Version))
	deadline := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	j, _, err := svc.Create(ctx, actor, moderation.CreateInput{URL: "https://example.test/deadline", Title: "Python engineer", Company: "Example", ApplicationDeadline: &deadline})
	if err != nil {
		t.Fatal(err)
	}
	slug := j.Fields().PublicSlug
	assertDeadline := func(want *time.Time) {
		t.Helper()
		row, err := q.GetJobBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		if row.ApplicationDeadline.Valid != (want != nil) || (want != nil && !row.ApplicationDeadline.Time.Equal(*want)) {
			t.Fatalf("deadline = %v, want %v", row.ApplicationDeadline, want)
		}
	}
	assertDeadline(&deadline)
	if count, err := q.CloseDeadlineExpiredJobs(ctx); err != nil || count != 0 {
		t.Fatalf("closed before deadline: %d %v", count, err)
	}
	title := "Senior Python engineer"
	if _, _, err := svc.Update(ctx, actor, slug, moderation.UpdatePatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	assertDeadline(&deadline)
	if _, _, err := svc.Update(ctx, actor, slug, moderation.UpdatePatch{ApplicationDeadlineSet: true}); err != nil {
		t.Fatal(err)
	}
	assertDeadline(nil)
	past := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	// Simulate time passing after a future deadline was saved.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET application_deadline=$1 WHERE public_slug=$2`, past, slug); err != nil {
		t.Fatal(err)
	}

	count, err := q.CloseDeadlineExpiredJobs(ctx)
	if err != nil || count != 1 {
		t.Fatalf("expiry: count=%d err=%v", count, err)
	}
	count, err = q.CloseDeadlineExpiredJobs(ctx)
	if err != nil || count != 0 {
		t.Fatalf("repeat expiry: count=%d err=%v", count, err)
	}
	row, err := q.GetJobBySlug(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	if !row.ClosedAt.Valid || !row.ClosedAt.Time.Equal(past) || row.ClosedReason != "application_deadline" {
		t.Fatalf("wrong close: %+v", row)
	}
	var queued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM search_delete_outbox WHERE job_id=$1`, row.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("removal queue=%d err=%v", queued, err)
	}
	if _, _, err := svc.Update(ctx, actor, slug, moderation.UpdatePatch{ApplicationDeadlineSet: true, ApplicationDeadline: &deadline}); err != nil {
		t.Fatal(err)
	}
	row, err = q.GetJobBySlug(ctx, slug)
	if err != nil || !row.ClosedAt.Valid {
		t.Fatalf("deadline edit reopened job: %v", err)
	}
}

func TestAutoApplyRefusesDeadlineBeforeSweep(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO jobs (source,external_id,url,title,public_slug,application_deadline) VALUES ('greenhouse','deadline','https://example.test/job','Engineer','expired-before-sweep',now()-interval '1 second')`); err != nil {
		t.Fatal(err)
	}
	h := &assistantHandlers{queries: db.New(pool)}
	err := h.startAutoApply(ctx, 1, "expired-before-sweep")
	var ferr *fiber.Error
	if !errors.As(err, &ferr) || ferr.Code != fiber.StatusGone {
		t.Fatalf("want 410 before CV or allowance check, got %v", err)
	}
}

func TestDeadlineEditCannotReopenBeforeSweep(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	q := db.New(pool)
	var actor int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email,role) VALUES ('deadline-edit@example.test','moderator') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	svc := moderation.New(moderation.NewQueriesRepository(q, pool, enrich.Version))
	past := time.Now().Add(-time.Hour)
	j, _, err := svc.Create(ctx, actor, moderation.CreateInput{URL: "https://example.test/expired", Title: "Engineer", Company: "Example", ApplicationDeadline: &past})
	if err != nil {
		t.Fatal(err)
	}
	if j.Fields().ClosedAt == nil {
		t.Fatal("historical publication must close immediately")
	}
	// A deadline can elapse between the last write and the next scheduled sweep.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET closed_at=NULL,closed_reason='' WHERE id=$1`, j.Fields().ID); err != nil {
		t.Fatal(err)
	}
	j, _, err = svc.Update(ctx, actor, j.Fields().PublicSlug, moderation.UpdatePatch{ApplicationDeadlineSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if j.Fields().ApplicationDeadline != nil || j.Fields().ClosedAt == nil {
		t.Fatal("clearing an elapsed deadline reopened the job before the sweep")
	}
}
