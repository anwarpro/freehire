package teamreview

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/pgconv"
	"github.com/strelov1/freehire/internal/platform/pgerr"
)

type StudentFollowupCreate struct {
	StudentRef  string `json:"student_ref"`
	StudentName string `json:"student_name"`
	ProfileURL  string `json:"profile_url"`
	Reason      string `json:"reason"`
}
type StudentFollowupUpdate struct {
	StudentName     string  `json:"student_name"`
	ProfileURL      string  `json:"profile_url"`
	Reason          string  `json:"reason"`
	Status          string  `json:"status"`
	Notes           string  `json:"notes"`
	FollowUp        *string `json:"follow_up"`
	ExpectedVersion int64   `json:"expected_version"`
}
type StudentContactInput struct {
	Channel         string `json:"channel"`
	ContactedAt     string `json:"contacted_at"`
	Note            string `json:"note"`
	RequestID       string `json:"request_id"`
	ExpectedVersion int64  `json:"expected_version"`
}
type StudentFollowupFilter struct {
	Slug, Status, Query string
	Overdue             bool
	Offset              int32
	ID                  int64
}
type StudentFollowupQueue struct {
	Data []db.ListStudentFollowupsRow `json:"data"`
	Meta struct {
		HasMore bool  `json:"has_more"`
		Limit   int   `json:"limit"`
		Offset  int32 `json:"offset"`
	} `json:"meta"`
}

func validStudentStage(s string) bool {
	switch s {
	case "suggested", "informed", "interested", "applied", "interview", "offer", "closed":
		return true
	}
	return false
}
func boundedText(s string, max int, required bool) bool {
	return (!required || strings.TrimSpace(s) != "") && utf8.RuneCountInString(s) <= max && !strings.ContainsRune(s, 0)
}
func validateStudentFields(name, profile, reason string) error {
	if !boundedText(name, 120, false) || strings.ContainsFunc(name, unicode.IsControl) || !boundedText(reason, 2000, true) || !boundedText(profile, 1000, false) {
		return fmt.Errorf("%w: use a name up to 120 characters, a profile link up to 1,000, and a reason of 1–2,000 characters", ErrInvalid)
	}
	if profile != "" {
		u, err := url.Parse(profile)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || strings.ContainsFunc(profile, unicode.IsSpace) {
			return fmt.Errorf("%w: profile link must be an HTTP or HTTPS URL without credentials", ErrInvalid)
		}
	}
	return nil
}

// Same lock order as assignments: current actor, public job, then child record.
// Job reassignment/privacy changes serialize with all follow-up writes.
func (s *Service) studentWrite(ctx context.Context, actor int64, slug string) (pgx.Tx, *db.Queries, db.GetTeamJobReviewRow, error) {
	tx, q, a, err := s.write(ctx, actor)
	if err != nil {
		return nil, nil, db.GetTeamJobReviewRow{}, err
	}
	if _, err = q.GetTeamJob(ctx, slug); err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, db.GetTeamJobReviewRow{}, err
	}
	job, err := q.GetTeamJobReview(ctx, db.GetTeamJobReviewParams{Today: today(), Slug: slug, IsAdmin: a.Role == "admin", ActorID: actor})
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, db.GetTeamJobReviewRow{}, err
	}
	return tx, q, job, nil
}
func (s *Service) StudentFollowups(ctx context.Context, actor int64, in StudentFollowupFilter) (StudentFollowupQueue, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return StudentFollowupQueue{}, err
	}
	if err = require(a, false); err != nil {
		return StudentFollowupQueue{}, err
	}
	if in.Offset < 0 || in.Offset > 100000 || !boundedText(in.Query, 300, false) || (in.Status != "" && !validStudentStage(in.Status)) {
		return StudentFollowupQueue{}, ErrInvalid
	}
	if in.Slug != "" {
		if _, err = s.Get(ctx, actor, in.Slug); err != nil {
			return StudentFollowupQueue{}, err
		}
	}
	rows, err := s.q.ListStudentFollowups(ctx, db.ListStudentFollowupsParams{Today: today(), FollowupID: in.ID, IsAdmin: a.Role == "admin", ActorID: actor, Slug: in.Slug, Status: in.Status, StudentQuery: strings.TrimSpace(in.Query), OverdueOnly: in.Overdue, PageLimit: 51, PageOffset: in.Offset})
	if err != nil {
		return StudentFollowupQueue{}, err
	}
	if rows == nil {
		rows = []db.ListStudentFollowupsRow{}
	}
	out := StudentFollowupQueue{Data: rows}
	out.Meta.Limit = 50
	out.Meta.Offset = in.Offset
	out.Meta.HasMore = len(rows) > 50
	if out.Meta.HasMore {
		out.Data = rows[:50]
	}
	return out, nil
}
func studentEvent(f db.TeamStudentFollowup, actor int64, action string) db.InsertStudentFollowupEventParams {
	return db.InsertStudentFollowupEventParams{FollowupID: f.ID, ActorID: pgconv.Int8(&actor), Action: action, StudentName: f.StudentName, ProfileUrl: f.ProfileUrl, Reason: f.Reason, Status: f.Status, Notes: f.Notes, FollowUp: f.FollowUp, Version: f.Version}
}
func (s *Service) CreateStudentFollowup(ctx context.Context, actor int64, slug string, in StudentFollowupCreate) (db.TeamStudentFollowup, error) {
	var zero db.TeamStudentFollowup
	in.StudentRef = strings.TrimSpace(in.StudentRef)
	in.StudentName = strings.TrimSpace(in.StudentName)
	in.ProfileURL = strings.TrimSpace(in.ProfileURL)
	in.Reason = strings.TrimSpace(in.Reason)
	tx, q, job, err := s.studentWrite(ctx, actor, slug)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if job.ClosedAt.Valid {
		return zero, fmt.Errorf("%w: this posting is closed; existing follow-ups remain available", ErrInvalid)
	}
	if !boundedText(in.StudentRef, 300, true) || strings.ContainsFunc(in.StudentRef, unicode.IsControl) {
		return zero, fmt.Errorf("%w: enter a student ID or reference of 1–300 characters", ErrInvalid)
	}
	if err = validateStudentFields(in.StudentName, in.ProfileURL, in.Reason); err != nil {
		return zero, err
	}
	f, err := q.CreateStudentFollowup(ctx, db.CreateStudentFollowupParams{JobID: job.JobID, StudentRef: in.StudentRef, StudentName: in.StudentName, ProfileUrl: in.ProfileURL, Reason: in.Reason, CreatedBy: pgconv.Int8(&actor)})
	if err != nil {
		if pgerr.IsUniqueViolation(err) {
			return zero, fmt.Errorf("%w: this student reference is already linked to this job", ErrExists)
		}
		return zero, err
	}
	if err = q.InsertStudentFollowupEvent(ctx, studentEvent(f, actor, "created")); err != nil {
		return zero, err
	}
	return f, tx.Commit(ctx)
}
func (s *Service) UpdateStudentFollowup(ctx context.Context, actor int64, slug string, id int64, in StudentFollowupUpdate) (db.TeamStudentFollowup, error) {
	var zero db.TeamStudentFollowup
	tx, q, job, err := s.studentWrite(ctx, actor, slug)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	f, err := q.GetStudentFollowup(ctx, db.GetStudentFollowupParams{ID: id, JobID: job.JobID})
	if err != nil {
		return zero, err
	}
	if f.Version != in.ExpectedVersion {
		return zero, ErrConflict
	}
	in.StudentName = strings.TrimSpace(in.StudentName)
	in.ProfileURL = strings.TrimSpace(in.ProfileURL)
	in.Reason = strings.TrimSpace(in.Reason)
	if err = validateStudentFields(in.StudentName, in.ProfileURL, in.Reason); err != nil {
		return zero, err
	}
	if !validStudentStage(in.Status) || !boundedText(in.Notes, 2000, false) {
		return zero, fmt.Errorf("%w: choose a follow-up outcome and keep notes within 2,000 characters", ErrInvalid)
	}
	date, err := followDate(in.FollowUp)
	if err != nil {
		return zero, err
	}
	if in.Status == "informed" {
		has, err := q.HasStudentContact(ctx, id)
		if err != nil {
			return zero, err
		}
		if !has {
			return zero, fmt.Errorf("%w: record manual contact before marking informed", ErrInvalid)
		}
	}
	if f.StudentName == in.StudentName && f.ProfileUrl == in.ProfileURL && f.Reason == in.Reason && f.Status == in.Status && f.Notes == in.Notes && f.FollowUp == date {
		return f, tx.Commit(ctx)
	}
	f, err = q.UpdateStudentFollowup(ctx, db.UpdateStudentFollowupParams{ID: id, StudentName: in.StudentName, ProfileUrl: in.ProfileURL, Reason: in.Reason, Status: in.Status, Notes: in.Notes, FollowUp: date, UpdatedBy: pgconv.Int8(&actor)})
	if err != nil {
		return zero, err
	}
	if err = q.InsertStudentFollowupEvent(ctx, studentEvent(f, actor, "updated")); err != nil {
		return zero, err
	}
	return f, tx.Commit(ctx)
}
func (s *Service) RecordStudentContact(ctx context.Context, actor int64, slug string, id int64, in StudentContactInput) (db.TeamStudentFollowup, error) {
	var zero db.TeamStudentFollowup
	tx, q, job, err := s.studentWrite(ctx, actor, slug)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	f, err := q.GetStudentFollowup(ctx, db.GetStudentFollowupParams{ID: id, JobID: job.JobID})
	if err != nil {
		return zero, err
	}
	switch in.Channel {
	case "email", "phone", "whatsapp", "messenger", "other":
	default:
		return zero, fmt.Errorf("%w: choose a contact channel", ErrInvalid)
	}
	at, err := time.Parse(time.RFC3339, in.ContactedAt)
	if err != nil || at.Year() < 1900 || at.After(time.Now().Add(5*time.Minute)) || !boundedText(in.Note, 2000, false) {
		return zero, fmt.Errorf("%w: enter a valid past contact time and a note up to 2,000 characters", ErrInvalid)
	}
	// PostgreSQL stores timestamps at microsecond precision; normalize retries too.
	at = at.UTC().Truncate(time.Microsecond)
	rid, err := uuid.Parse(in.RequestID)
	if err != nil || rid == uuid.Nil {
		return zero, fmt.Errorf("%w: a contact request ID is required", ErrInvalid)
	}
	requestID := pgtype.Text{String: rid.String(), Valid: true}
	existing, err := q.GetStudentContactRequest(ctx, db.GetStudentContactRequestParams{FollowupID: id, RequestID: requestID})
	if err == nil {
		if !existing.ActorID.Valid || existing.ActorID.Int64 != actor || existing.Channel != in.Channel || !existing.ContactedAt.Time.Equal(at) || existing.ContactNote != in.Note {
			return zero, ErrConflict
		}
		return f, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	if f.Version != in.ExpectedVersion {
		return zero, ErrConflict
	}
	if job.ClosedAt.Valid || f.Status == "closed" {
		return zero, fmt.Errorf("%w: contact cannot be recorded for a closed posting or closed follow-up", ErrInvalid)
	}
	stage := f.Status
	if stage == "suggested" {
		stage = "informed"
	}
	f, err = q.UpdateStudentFollowup(ctx, db.UpdateStudentFollowupParams{ID: id, StudentName: f.StudentName, ProfileUrl: f.ProfileUrl, Reason: f.Reason, Status: stage, Notes: f.Notes, FollowUp: f.FollowUp, UpdatedBy: pgconv.Int8(&actor)})
	if err != nil {
		return zero, err
	}
	event := studentEvent(f, actor, "contacted")
	event.Channel = in.Channel
	event.ContactedAt = pgtype.Timestamptz{Time: at, Valid: true}
	event.ContactNote = in.Note
	event.RequestID = requestID
	if err = q.InsertStudentFollowupEvent(ctx, event); err != nil {
		return zero, err
	}
	return f, tx.Commit(ctx)
}
func (s *Service) StudentFollowupHistory(ctx context.Context, actor int64, slug string, id int64) ([]db.ListStudentFollowupEventsRow, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = require(a, false); err != nil {
		return nil, err
	}
	rows, err := s.q.ListStudentFollowupEvents(ctx, db.ListStudentFollowupEventsParams{FollowupID: id, Slug: slug, IsAdmin: a.Role == "admin", ActorID: actor})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, pgx.ErrNoRows
	}
	return rows, nil
}

func (s *Service) StudentFollowup(ctx context.Context, actor int64, slug string, id int64) (db.ListStudentFollowupsRow, error) {
	out, err := s.StudentFollowups(ctx, actor, StudentFollowupFilter{Slug: slug, ID: id})
	if err != nil {
		return db.ListStudentFollowupsRow{}, err
	}
	if len(out.Data) != 1 {
		return db.ListStudentFollowupsRow{}, pgx.ErrNoRows
	}
	return out.Data[0], nil
}
