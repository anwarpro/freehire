package teamreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/strelov1/freehire/internal/job/curriculum"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/pgconv"
)

type CurriculumSettings struct {
	Version  int64                  `json:"version"`
	Config   curriculum.Config      `json:"config"`
	Progress db.CurriculumScanState `json:"progress"`
}
type CurriculumConfigInput struct {
	ExpectedVersion int64             `json:"expected_version"`
	Config          curriculum.Config `json:"config"`
}
type CurriculumReviewInput struct {
	Tracks          []string `json:"tracks"`
	Notes           string   `json:"notes"`
	ExpectedVersion int64    `json:"expected_version"`
	MappingVersion  int64    `json:"mapping_version"`
	InputHash       string   `json:"input_hash"`
}
type CurriculumDetail struct {
	db.CurriculumMatch
	Tracks      []curriculum.Track `json:"tracks"`
	Reviewed    bool               `json:"reviewed"`
	ReviewStale bool               `json:"review_stale"`
}
type CurriculumQueueInput struct {
	Track, Source, State, Review string
	Offset                       int32
}
type CurriculumQueue struct {
	Data []db.ListCurriculumJobsRow `json:"data"`
	Meta struct {
		HasMore bool  `json:"has_more"`
		Offset  int32 `json:"offset"`
		Limit   int   `json:"limit"`
	} `json:"meta"`
}

func (s *Service) CurriculumSettings(ctx context.Context, actor int64) (CurriculumSettings, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return CurriculumSettings{}, err
	}
	if err = require(a, false); err != nil {
		return CurriculumSettings{}, err
	}
	cfg, err := s.q.GetCurriculumConfig(ctx)
	if err != nil {
		return CurriculumSettings{}, err
	}
	config, err := curriculum.Decode(cfg.Payload)
	if err != nil {
		return CurriculumSettings{}, err
	}
	progress, err := s.q.GetCurriculumScan(ctx)
	return CurriculumSettings{Version: cfg.Version, Config: config, Progress: progress}, err
}
func (s *Service) SaveCurriculumSettings(ctx context.Context, actor int64, in CurriculumConfigInput) (CurriculumSettings, error) {
	for ti := range in.Config.Tracks {
		t := &in.Config.Tracks[ti]
		t.Label = strings.TrimSpace(t.Label)
		for si := range t.Skills {
			sk := &t.Skills[si]
			sk.Name = strings.TrimSpace(sk.Name)
			for ai := range sk.Aliases {
				sk.Aliases[ai] = strings.TrimSpace(sk.Aliases[ai])
			}
		}
	}
	tx, q, a, err := s.write(ctx, actor)
	if err != nil {
		return CurriculumSettings{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = require(a, true); err != nil {
		return CurriculumSettings{}, err
	}
	if _, err := curriculum.New(in.Config); err != nil {
		return CurriculumSettings{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	old, err := q.GetCurriculumConfig(ctx)
	if err != nil {
		return CurriculumSettings{}, err
	}
	if old.Version != in.ExpectedVersion {
		return CurriculumSettings{}, ErrConflict
	}
	previous, err := curriculum.Decode(old.Payload)
	if err != nil {
		return CurriculumSettings{}, err
	}
	for _, t := range previous.Tracks {
		found := false
		for _, n := range in.Config.Tracks {
			if n.ID == t.ID {
				found = true
				break
			}
		}
		if !found {
			return CurriculumSettings{}, fmt.Errorf("%w: disable a track instead of removing its stable ID", ErrInvalid)
		}
	}
	raw, err := json.Marshal(in.Config)
	if err != nil {
		return CurriculumSettings{}, err
	}
	oldRaw, _ := json.Marshal(previous)
	if string(raw) != string(oldRaw) {
		updated, err := q.UpdateCurriculumConfig(ctx, db.UpdateCurriculumConfigParams{Payload: raw, UpdatedBy: pgconv.Int8(&actor), Version: in.ExpectedVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return CurriculumSettings{}, ErrConflict
		}
		if err != nil {
			return CurriculumSettings{}, err
		}
		if err = q.InsertCurriculumConfigEvent(ctx, db.InsertCurriculumConfigEventParams{Version: updated.Version, Payload: raw, ActorID: pgconv.Int8(&actor)}); err != nil {
			return CurriculumSettings{}, err
		}
		if err = q.ResetCurriculumScan(ctx, updated.Version); err != nil {
			return CurriculumSettings{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return CurriculumSettings{}, err
	}
	return s.CurriculumSettings(ctx, actor)
}
func (s *Service) CurriculumConfigHistory(ctx context.Context, actor int64) ([]db.ListCurriculumConfigEventsRow, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = require(a, true); err != nil {
		return nil, err
	}
	return s.q.ListCurriculumConfigEvents(ctx)
}
func detail(m db.CurriculumMatch, c curriculum.Config) CurriculumDetail {
	return CurriculumDetail{CurriculumMatch: m, Tracks: c.Tracks, Reviewed: m.ReviewedTracks != nil, ReviewStale: m.ReviewedTracks != nil && (m.ReviewedHash.String != m.InputHash || m.ReviewedMappingVersion.Int64 != m.MappingVersion)}
}
func (s *Service) CurriculumJob(ctx context.Context, actor int64, slug string) (CurriculumDetail, error) {
	tx, q, a, err := s.write(ctx, actor)
	if err != nil {
		return CurriculumDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := q.GetTeamJob(ctx, slug)
	if err != nil {
		return CurriculumDetail{}, err
	}
	if _, err = q.GetTeamJobReview(ctx, db.GetTeamJobReviewParams{Today: today(), Slug: slug, IsAdmin: a.Role == "admin", ActorID: actor}); err != nil {
		return CurriculumDetail{}, err
	}
	m, err := curriculum.Refresh(ctx, tx, id)
	if err != nil {
		return CurriculumDetail{}, err
	}
	cfg, err := q.GetCurriculumConfig(ctx)
	if err != nil {
		return CurriculumDetail{}, err
	}
	c, err := curriculum.Decode(cfg.Payload)
	if err != nil {
		return CurriculumDetail{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CurriculumDetail{}, err
	}
	return detail(m, c), nil
}
func (s *Service) ReviewCurriculum(ctx context.Context, actor int64, slug string, in CurriculumReviewInput) (CurriculumDetail, error) {
	if utf8.RuneCountInString(in.Notes) > 2000 || strings.ContainsRune(in.Notes, 0) || in.ExpectedVersion < 0 {
		return CurriculumDetail{}, ErrInvalid
	}
	tx, q, a, err := s.write(ctx, actor)
	if err != nil {
		return CurriculumDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := q.GetTeamJob(ctx, slug)
	if err != nil {
		return CurriculumDetail{}, err
	}
	if _, err = q.GetTeamJobReview(ctx, db.GetTeamJobReviewParams{Today: today(), Slug: slug, IsAdmin: a.Role == "admin", ActorID: actor}); err != nil {
		return CurriculumDetail{}, err
	}
	m, err := curriculum.Refresh(ctx, tx, id)
	if err != nil {
		return CurriculumDetail{}, err
	}
	if m.ReviewVersion != in.ExpectedVersion || m.MappingVersion != in.MappingVersion || m.InputHash != in.InputHash {
		return CurriculumDetail{}, ErrConflict
	}
	cfg, err := q.GetCurriculumConfig(ctx)
	if err != nil {
		return CurriculumDetail{}, err
	}
	c, err := curriculum.Decode(cfg.Payload)
	if err != nil {
		return CurriculumDetail{}, err
	}
	enabled := map[string]bool{}
	for _, t := range c.Tracks {
		enabled[t.ID] = t.Enabled
	}
	seen := map[string]bool{}
	for _, id := range in.Tracks {
		if !enabled[id] || seen[id] {
			return CurriculumDetail{}, fmt.Errorf("%w: choose enabled curriculum tracks once each", ErrInvalid)
		}
		seen[id] = true
	}
	slices.Sort(in.Tracks)
	if (in.Tracks == nil) != (m.ReviewedTracks == nil) || !slices.Equal(in.Tracks, m.ReviewedTracks) || m.ReviewNotes != in.Notes || detail(m, c).ReviewStale {
		m, err = q.ReviewCurriculumMatch(ctx, db.ReviewCurriculumMatchParams{JobID: id, ReviewedTracks: in.Tracks, ReviewedBy: pgconv.Int8(&actor), ReviewNotes: in.Notes})
		if err != nil {
			return CurriculumDetail{}, err
		}
		if err = q.InsertCurriculumReviewEvent(ctx, db.InsertCurriculumReviewEventParams{JobID: id, ActorID: pgconv.Int8(&actor), Tracks: in.Tracks, Notes: in.Notes, InputHash: m.InputHash, MappingVersion: m.MappingVersion, Version: m.ReviewVersion}); err != nil {
			return CurriculumDetail{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return CurriculumDetail{}, err
	}
	return detail(m, c), nil
}
func (s *Service) CurriculumHistory(ctx context.Context, actor int64, slug string) ([]db.ListCurriculumReviewEventsRow, error) {
	review, err := s.Get(ctx, actor, slug)
	if err != nil {
		return nil, err
	}
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	return s.q.ListCurriculumReviewEvents(ctx, db.ListCurriculumReviewEventsParams{JobID: review.JobID, IsAdmin: a.Role == "admin", ActorID: pgconv.Int8(&actor)})
}
func (s *Service) CurriculumJobs(ctx context.Context, actor int64, in CurriculumQueueInput) (CurriculumQueue, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return CurriculumQueue{}, err
	}
	if err = require(a, false); err != nil {
		return CurriculumQueue{}, err
	}
	if in.State == "" {
		in.State = "active"
	}
	if !slices.Contains([]string{"active", "closed", "all"}, in.State) || !slices.Contains([]string{"", "unreviewed", "reviewed", "needs_review"}, in.Review) || len(in.Source) > 80 || in.Offset < 0 || in.Offset > 100000 {
		return CurriculumQueue{}, ErrInvalid
	}
	cfg, err := s.q.GetCurriculumConfig(ctx)
	if err != nil {
		return CurriculumQueue{}, err
	}
	c, err := curriculum.Decode(cfg.Payload)
	if err != nil {
		return CurriculumQueue{}, err
	}
	if in.Track != "" {
		found := false
		for _, t := range c.Tracks {
			found = found || t.ID == in.Track
		}
		if !found {
			return CurriculumQueue{}, ErrInvalid
		}
	}
	rows, err := s.q.ListCurriculumJobs(ctx, db.ListCurriculumJobsParams{MappingVersion: cfg.Version, IsAdmin: a.Role == "admin", ActorID: pgconv.Int8(&actor), Track: in.Track, Source: in.Source, JobState: in.State, ReviewState: in.Review, PageOffset: in.Offset, PageLimit: 51})
	out := CurriculumQueue{Data: rows}
	out.Meta.Offset = in.Offset
	out.Meta.Limit = 50
	out.Meta.HasMore = len(rows) > 50
	if out.Meta.HasMore {
		out.Data = rows[:50]
	}
	return out, err
}
