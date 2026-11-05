package curriculum

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/strelov1/freehire/internal/platform/db"
)

var compiledCache struct {
	sync.Mutex
	key    [32]byte
	engine *Engine
}

func cachedEngine(raw []byte) (*Engine, error) {
	key := sha256.Sum256(raw)
	compiledCache.Lock()
	defer compiledCache.Unlock()
	if compiledCache.engine != nil && compiledCache.key == key {
		return compiledCache.engine, nil
	}
	cfg, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	engine, err := New(cfg)
	if err == nil {
		compiledCache.key = key
		compiledCache.engine = engine
	}
	return engine, err
}

// Refresh serializes on the job before config and projection access. Ingest uses
// the same job→outbox lock order, so edits cannot lose their queue entry or be
// overwritten by a computation of an older description. Caller owns the transaction.
func Refresh(ctx context.Context, tx pgx.Tx, jobID int64) (db.CurriculumMatch, error) {
	q := db.New(tx)
	j, err := q.LockCurriculumJob(ctx, jobID)
	if err != nil {
		return db.CurriculumMatch{}, err
	}
	if j.IsPrivate {
		if err = q.DeleteCurriculumMatch(ctx, jobID); err == nil {
			err = q.DeleteCurriculumDirty(ctx, jobID)
		}
		if err != nil {
			return db.CurriculumMatch{}, err
		}
		return db.CurriculumMatch{}, pgx.ErrNoRows
	}
	cfg, err := q.LockCurriculumConfig(ctx)
	if err != nil {
		return db.CurriculumMatch{}, err
	}
	hash := Hash(j.Title, j.Description)
	old, err := q.GetCurriculumMatch(ctx, jobID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return old, err
	}
	if err != nil || old.MappingVersion != cfg.Version || old.InputHash != hash {
		engine, err := cachedEngine(cfg.Payload)
		if err != nil {
			return old, err
		}
		result := engine.Match(j.Title, j.Description)
		raw, err := json.Marshal(result)
		if err != nil {
			return old, err
		}
		old, err = q.UpsertCurriculumMatch(ctx, db.UpsertCurriculumMatchParams{JobID: jobID, MappingVersion: cfg.Version, InputHash: hash, Evidence: raw, SuggestedTracks: result.TrackIDs()})
		if err != nil {
			return old, err
		}
	}
	err = q.DeleteCurriculumDirty(ctx, jobID)
	return old, err
}

// Drain processes dirty writes and a bounded seed scan fairly. The database
// trigger owns edit delivery; the scan supplies existing jobs and mapping changes.
func Drain(ctx context.Context, pool *pgxpool.Pool, maxJobs int) (int, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	const lockID = int64(0x637572726963) // "curric", session advisory lock for this worker
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&locked); err != nil || !locked {
		return 0, err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", lockID)
	}()
	q := db.New(pool)
	processed := 0
	process := func(id int64) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = Refresh(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			err = db.New(tx).DeleteCurriculumDirty(ctx, id)
		}
		if err != nil {
			return err
		}
		if err = tx.Commit(ctx); err == nil {
			processed++
		}
		return err
	}
	for processed < maxJobs {
		dirty, err := q.ListCurriculumDirty(ctx, 100)
		if err != nil {
			return processed, err
		}
		worked := false
		for _, id := range dirty {
			if processed >= maxJobs {
				break
			}
			if err = process(id); err != nil {
				return processed, err
			}
			worked = true
		}
		state, err := q.GetCurriculumScan(ctx)
		if err != nil {
			return processed, err
		}
		if !state.Complete && processed < maxJobs {
			seedVersion := state.MappingVersion
			if err = q.InitializeCurriculumScan(ctx, seedVersion); err != nil {
				return processed, err
			}
			state, err = q.GetCurriculumScan(ctx)
			if err != nil {
				return processed, err
			}
			// A manager may reset the state while initialization waits on its
			// row lock. Initialize the NEW generation on the next pass before
			// interpreting its zero upper bound as an empty catalogue.
			if state.MappingVersion != seedVersion || state.Complete {
				continue
			}
			ids, err := q.ListCurriculumSeed(ctx, db.ListCurriculumSeedParams{CursorID: state.CursorID, UpperID: state.UpperID, PageLimit: 100})
			if err != nil {
				return processed, err
			}
			cursor := state.CursorID
			for _, id := range ids {
				if processed >= maxJobs {
					break
				}
				if err = process(id); err != nil {
					return processed, err
				}
				cursor = id
				worked = true
			}
			complete := len(ids) == 0
			if complete {
				cursor = state.UpperID
			}
			if err = q.AdvanceCurriculumScan(ctx, db.AdvanceCurriculumScanParams{MappingVersion: state.MappingVersion, CursorID: cursor, Complete: complete}); err != nil {
				return processed, err
			}
		}
		if !worked {
			break
		}
	}
	return processed, nil
}

// RecordRun uses a short independent context so a time-budgeted run can still
// publish its progress. Timeout/cancellation is a resumable partial pass.
func RecordRun(pool *pgxpool.Pool, processed int, runErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message := ""
	if runErr != nil && !errors.Is(runErr, context.DeadlineExceeded) && !errors.Is(runErr, context.Canceled) {
		message = "Curriculum processing failed; inspect worker logs"
	}
	return db.New(pool).RecordCurriculumRun(ctx, db.RecordCurriculumRunParams{Processed: int64(processed), LastError: message})
}
