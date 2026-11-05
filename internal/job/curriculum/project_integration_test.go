//go:build integration

package curriculum

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/testdb"
)

func TestProjectionQueuesCommittedEditsAndPreservesManualLabels(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO jobs(source,external_id,url,title,description,public_slug) VALUES('curriculum','1','https://example.test/1','Engineer','Python required.','curriculum-one') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := Drain(ctx, pool, 10); err != nil {
		t.Fatal(err)
	}
	old, err := q.GetCurriculumMatch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE curriculum_matches SET reviewed_tracks=ARRAY['l1'],reviewed_hash=input_hash,reviewed_mapping_version=mapping_version,review_version=1 WHERE job_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	// Leave a visible dirty entry, then edit while holding the job lock. The worker
	// must observe the committed new body, never consume an edit it did not read.
	if _, err := pool.Exec(ctx, `INSERT INTO curriculum_outbox(job_id) VALUES($1) ON CONFLICT DO NOTHING`, id); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE jobs SET description='Django and machine learning required.' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); close(started); _, err := Drain(ctx, pool, 10); done <- err }()
	<-started
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	next, err := q.GetCurriculumMatch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if next.InputHash == old.InputHash || next.InputHash != Hash("Engineer", "Django and machine learning required.") {
		t.Fatal("worker consumed an edit without matching its body")
	}
	if len(next.ReviewedTracks) != 1 || next.ReviewedTracks[0] != "l1" || next.ReviewVersion != 1 {
		t.Fatal("worker overwrote manual review")
	}
	dirty, err := q.ListCurriculumDirty(ctx, 10)
	if err != nil || len(dirty) != 0 {
		t.Fatalf("queue not drained: %v %v", dirty, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE jobs SET description='FastAPI required.' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	dirty, err = q.ListCurriculumDirty(ctx, 10)
	if err != nil || len(dirty) != 1 {
		t.Fatal("subsequent edit not queued")
	}
	if _, err = Drain(ctx, pool, 10); err != nil {
		t.Fatal(err)
	}
	final, err := q.GetCurriculumMatch(ctx, id)
	if err != nil || final.InputHash != Hash("Engineer", "FastAPI required.") {
		t.Fatalf("subsequent edit lost: %v", err)
	}
}

func TestReopeningAnUnprojectedJobQueuesMatching(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO jobs(source,external_id,url,title,description,public_slug,closed_at) VALUES('curriculum','closed','https://example.test/closed','Engineer','Python required.','curriculum-closed',now()) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	// Simulate a posting already closed before curriculum matching was introduced.
	if _, err := pool.Exec(ctx, `DELETE FROM curriculum_outbox WHERE job_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := Drain(ctx, pool, 10); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	if _, err := q.GetCurriculumMatch(ctx, id); err == nil {
		t.Fatal("closed historical posting should not enter initial seed")
	}
	if _, err := pool.Exec(ctx, `UPDATE jobs SET closed_at=NULL WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := Drain(ctx, pool, 10); err != nil {
		t.Fatal(err)
	}
	result, err := q.GetCurriculumMatch(ctx, id)
	if err != nil || len(result.SuggestedTracks) == 0 {
		t.Fatalf("reopened job never matched: %+v %v", result, err)
	}
}

func TestMappingResetDuringSeedInitializationDoesNotSkipCatalogue(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	q := db.New(pool)
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO jobs(source,external_id,url,title,description,public_slug) VALUES('curriculum','reset-race','https://example.test/reset-race','Engineer','Python required.','curriculum-reset-race') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := Drain(ctx, pool, 10); err != nil {
		t.Fatal(err)
	}
	if err := q.ResetCurriculumScan(ctx, 1); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE curriculum_config SET version=2 WHERE singleton`); err != nil {
		t.Fatal(err)
	}
	if err = db.New(tx).ResetCurriculumScan(ctx, 2); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := Drain(ctx, pool, 10); done <- err }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%UPDATE curriculum_scan_state SET upper_id=%')`).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker never reached the seed initialization lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	match, err := q.GetCurriculumMatch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	state, err := q.GetCurriculumScan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if match.MappingVersion != 2 || !state.Complete || state.CursorID < id {
		t.Fatalf("new mapping falsely completed without matching: match=%d state=%+v", match.MappingVersion, state)
	}
}
