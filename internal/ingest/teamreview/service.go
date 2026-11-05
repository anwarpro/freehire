// Package teamreview owns private staff membership and job-review assignments.
// It never changes a public posting or grants a reviewer a global account role.
package teamreview

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/strelov1/freehire/internal/identity/accounts"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/pgconv"
)

var (
	ErrForbidden = errors.New("Team access is required")
	ErrConflict  = errors.New("This record changed. Reload it before saving")
	ErrInvalid   = errors.New("Invalid team input")
	ErrExists    = errors.New("This account or team member already exists")
)

type Access struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}
type MemberInput struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	UseExisting bool   `json:"use_existing"`
}
type MemberUpdate struct {
	Name            string `json:"name"`
	Active          bool   `json:"active"`
	ExpectedVersion int64  `json:"expected_version"`
}
type ReviewInput struct {
	AssigneeID      *int64  `json:"assignee_id"`
	Status          string  `json:"status"`
	Notes           string  `json:"notes"`
	FollowUp        *string `json:"follow_up"`
	ExpectedVersion int64   `json:"expected_version"`
}
type QueueInput struct {
	Scope   string
	Status  string
	Overdue bool
	Offset  int32
}
type Queue struct {
	Data []db.ListTeamReviewsRow `json:"data"`
	Meta struct {
		HasMore bool  `json:"has_more"`
		Offset  int32 `json:"offset"`
		Limit   int   `json:"limit"`
	} `json:"meta"`
}
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	hasher accounts.PasswordHasher
}

func New(pool *pgxpool.Pool, q *db.Queries, hasher accounts.PasswordHasher) *Service {
	return &Service{pool: pool, q: q, hasher: hasher}
}

func access(ctx context.Context, q *db.Queries, id int64) (Access, error) {
	r, err := q.GetTeamAccess(ctx, id)
	return Access{UserID: r.ID, Name: r.Name, Role: r.TeamRole}, err
}
func (s *Service) Access(ctx context.Context, id int64) (Access, error) { return access(ctx, s.q, id) }
func require(a Access, admin bool) error {
	if a.Role == "none" || a.Role == "" || (admin && a.Role != "admin") {
		return ErrForbidden
	}
	return nil
}

func (s *Service) write(ctx context.Context, actor int64, others ...int64) (pgx.Tx, *db.Queries, Access, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, Access{}, err
	}
	q := s.q.WithTx(tx)
	ids := append(others, actor)
	_, err = q.LockTeamUsers(ctx, ids)
	var a Access
	if err == nil {
		a, err = access(ctx, q, actor)
	}
	if err == nil {
		err = require(a, false)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, Access{}, err
	}
	return tx, q, a, nil
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 120 || strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("%w: name must contain 1–120 characters", ErrInvalid)
	}
	return nil
}
func validStatus(status string) bool {
	return status == "pending" || status == "in_review" || status == "reviewed" || status == "not_relevant"
}
func followDate(value *string) (pgtype.Date, error) {
	if value == nil {
		return pgtype.Date{}, nil
	}
	d, err := time.Parse("2006-01-02", *value)
	if err != nil || d.Year() < 1900 || d.Year() > 9999 {
		return pgtype.Date{}, fmt.Errorf("%w: follow-up must be a valid calendar date", ErrInvalid)
	}
	return pgtype.Date{Time: d, Valid: true}, nil
}
func today() pgtype.Date {
	d := time.Now().In(time.FixedZone("Asia/Dhaka", 6*60*60)).Format("2006-01-02")
	t, _ := time.Parse("2006-01-02", d)
	return pgtype.Date{Time: t, Valid: true}
}

func (s *Service) Members(ctx context.Context, actor int64) ([]db.ListTeamMembersRow, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = require(a, true); err != nil {
		return nil, err
	}
	rows, err := s.q.ListTeamMembers(ctx)
	if len(rows) > 500 {
		return nil, fmt.Errorf("%w: team exceeds the 500-member workspace limit", ErrInvalid)
	}
	return rows, err
}

func (s *Service) CreateMember(ctx context.Context, actor int64, in MemberInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := validateName(in.Name); err != nil {
		return 0, err
	}
	a0, err := s.Access(ctx, actor)
	if err != nil {
		return 0, err
	}
	if err = require(a0, true); err != nil {
		return 0, err
	}
	var existingID int64
	if in.UseExisting {
		if in.Password != "" {
			return 0, fmt.Errorf("%w: existing accounts keep their own password", ErrInvalid)
		}
		u, err := s.q.GetUserByEmail(ctx, strings.TrimSpace(in.Email))
		if err != nil {
			return 0, err
		}
		existingID = u.ID
	}
	tx, q, a, err := s.write(ctx, actor, existingID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = require(a, true); err != nil {
		return 0, err
	}
	id := existingID
	if in.UseExisting {
		u, err := q.GetUserByEmail(ctx, strings.TrimSpace(in.Email))
		if err != nil {
			return 0, err
		}
		if u.ID != existingID || !u.EmailVerified {
			return 0, fmt.Errorf("%w: only a verified existing account can join the team", ErrInvalid)
		}
		if u.Role == "admin" || u.Role == "moderator" {
			return 0, ErrExists
		}
	} else {
		zone := "Asia/Dhaka"
		// Registration policy and bcrypt are shared with public sign-up. With no
		// mailer attached, account creation sends no message or invitation.
		accountService := accounts.New(accounts.NewQueriesRepository(s.q, s.pool).WithTx(tx), s.hasher)
		u, err := accountService.Register(ctx, in.Email, in.Password, &zone)
		if err != nil {
			if errors.Is(err, accounts.ErrEmailTaken) {
				return 0, ErrExists
			}
			if errors.Is(err, accounts.ErrInvalidEmail) || errors.Is(err, accounts.ErrPasswordTooShort) || errors.Is(err, accounts.ErrPasswordTooLong) {
				return 0, fmt.Errorf("%w: use a valid email and an 8–72 byte password", ErrInvalid)
			}
			return 0, err
		}
		id = u.ID
	}
	if _, err = q.GetTeamMember(ctx, id); err == nil {
		return 0, ErrExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if _, err = q.CreateTeamMember(ctx, db.CreateTeamMemberParams{UserID: id, Name: in.Name, CreatedBy: pgconv.Int8(&actor)}); err != nil {
		return 0, err
	}
	action := "created"
	if in.UseExisting {
		action = "added"
	}
	if err = q.InsertTeamMemberEvent(ctx, db.InsertTeamMemberEventParams{MemberID: pgconv.Int8(&id), ActorID: pgconv.Int8(&actor), Action: action}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s *Service) UpdateMember(ctx context.Context, actor, id int64, in MemberUpdate) error {
	in.Name = strings.TrimSpace(in.Name)
	if err := validateName(in.Name); err != nil {
		return err
	}
	if in.ExpectedVersion < 1 {
		return ErrConflict
	}
	tx, q, a, err := s.write(ctx, actor, id)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = require(a, true); err != nil {
		return err
	}
	m, err := q.GetTeamMember(ctx, id)
	if err != nil {
		return err
	}
	if m.Role != "user" {
		return ErrForbidden
	}
	if m.Version != in.ExpectedVersion {
		return ErrConflict
	}
	if m.Name == in.Name && m.Active == in.Active {
		return tx.Commit(ctx)
	}
	if _, err = q.UpdateTeamMember(ctx, db.UpdateTeamMemberParams{UserID: id, Name: in.Name, Active: in.Active, Version: in.ExpectedVersion}); err != nil {
		return err
	}
	action := "updated"
	if m.Active != in.Active {
		action = "deactivated"
		if in.Active {
			action = "activated"
		}
	}
	if err = q.InsertTeamMemberEvent(ctx, db.InsertTeamMemberEventParams{MemberID: pgconv.Int8(&id), ActorID: pgconv.Int8(&actor), Action: action}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) MemberHistory(ctx context.Context, actor, id int64) ([]db.ListTeamMemberEventsRow, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = require(a, true); err != nil {
		return nil, err
	}
	if _, err = s.q.GetTeamMember(ctx, id); err != nil {
		return nil, err
	}
	return s.q.ListTeamMemberEvents(ctx, pgconv.Int8(&id))
}

func (s *Service) Get(ctx context.Context, actor int64, slug string) (db.GetTeamJobReviewRow, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return db.GetTeamJobReviewRow{}, err
	}
	if err = require(a, false); err != nil {
		return db.GetTeamJobReviewRow{}, err
	}
	return s.q.GetTeamJobReview(ctx, db.GetTeamJobReviewParams{Today: today(), Slug: slug, IsAdmin: a.Role == "admin", ActorID: actor})
}
func (s *Service) List(ctx context.Context, actor int64, in QueueInput) (Queue, error) {
	a, err := s.Access(ctx, actor)
	if err != nil {
		return Queue{}, err
	}
	if err = require(a, false); err != nil {
		return Queue{}, err
	}
	if in.Scope == "" {
		in.Scope = "mine"
		if a.Role == "admin" {
			in.Scope = "all"
		}
	}
	if (in.Scope != "mine" && in.Scope != "all" && in.Scope != "unassigned") || (in.Status != "" && !validStatus(in.Status)) || in.Offset < 0 || in.Offset > 100000 {
		return Queue{}, ErrInvalid
	}
	if a.Role != "admin" && in.Scope != "mine" {
		return Queue{}, ErrForbidden
	}
	rows, err := s.q.ListTeamReviews(ctx, db.ListTeamReviewsParams{Today: today(), IsAdmin: a.Role == "admin", ActorID: actor, Scope: in.Scope, Status: in.Status, OverdueOnly: in.Overdue, PageLimit: 51, PageOffset: in.Offset})
	if err != nil {
		return Queue{}, err
	}
	out := Queue{Data: rows}
	out.Meta.Limit = 50
	out.Meta.Offset = in.Offset
	out.Meta.HasMore = len(rows) > 50
	if out.Meta.HasMore {
		out.Data = rows[:50]
	}
	return out, nil
}

func (s *Service) Save(ctx context.Context, actor int64, slug string, in ReviewInput) (db.GetTeamJobReviewRow, error) {
	var zero db.GetTeamJobReviewRow
	if !validStatus(in.Status) || utf8.RuneCountInString(in.Notes) > 5000 || in.ExpectedVersion < 0 || strings.ContainsRune(in.Notes, 0) {
		return zero, fmt.Errorf("%w: choose a review status and keep notes within 5,000 characters", ErrInvalid)
	}
	date, err := followDate(in.FollowUp)
	if err != nil {
		return zero, err
	}
	ids := []int64{}
	if in.AssigneeID != nil {
		if *in.AssigneeID <= 0 {
			return zero, ErrInvalid
		}
		ids = append(ids, *in.AssigneeID)
	}
	tx, q, a, err := s.write(ctx, actor, ids...)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	jobID, err := q.GetTeamJob(ctx, slug)
	if err != nil {
		return zero, err
	}
	old, err := q.GetTeamReviewRecord(ctx, jobID)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	if a.Role != "admin" && (!exists || !old.AssigneeID.Valid || old.AssigneeID.Int64 != actor) {
		return zero, pgx.ErrNoRows
	}
	if a.Role != "admin" && (in.AssigneeID == nil || *in.AssigneeID != actor) {
		return zero, ErrForbidden
	}
	if (!exists && in.ExpectedVersion != 0) || (exists && old.Version != in.ExpectedVersion) {
		return zero, ErrConflict
	}
	if in.AssigneeID != nil {
		target, err := access(ctx, q, *in.AssigneeID)
		if err != nil {
			return zero, err
		}
		if target.Role == "none" {
			return zero, fmt.Errorf("%w: choose an active team member", ErrInvalid)
		}
	}
	assigned := pgconv.Int8(in.AssigneeID)
	if exists && old.AssigneeID == assigned && old.Status == in.Status && old.Notes == in.Notes && old.FollowUp == date {
		if err = tx.Commit(ctx); err != nil {
			return zero, err
		}
		return s.Get(ctx, actor, slug)
	}
	r, err := q.SaveTeamReview(ctx, db.SaveTeamReviewParams{JobID: jobID, AssigneeID: assigned, Status: in.Status, Notes: in.Notes, FollowUp: date, UpdatedBy: pgconv.Int8(&actor)})
	if err != nil {
		return zero, err
	}
	action := "updated"
	if !exists && assigned.Valid {
		action = "assigned"
	} else if exists && old.AssigneeID != assigned {
		action = "reassigned"
		if !assigned.Valid {
			action = "unassigned"
		} else if !old.AssigneeID.Valid {
			action = "assigned"
		}
	}
	if err = q.InsertTeamReviewEvent(ctx, db.InsertTeamReviewEventParams{JobID: jobID, ActorID: pgconv.Int8(&actor), AssigneeID: assigned, Action: action, Status: in.Status, Notes: in.Notes, FollowUp: date, Version: r.Version}); err != nil {
		return zero, err
	}
	if err = tx.Commit(ctx); err != nil {
		return zero, err
	}
	return s.Get(ctx, actor, slug)
}

func (s *Service) History(ctx context.Context, actor int64, slug string) ([]db.ListTeamReviewEventsRow, error) {
	r, err := s.Get(ctx, actor, slug)
	if err != nil {
		return nil, err
	}
	a, err := s.Access(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err = require(a, false); err != nil {
		return nil, err
	}
	return s.q.ListTeamReviewEvents(ctx, db.ListTeamReviewEventsParams{JobID: r.JobID, IsAdmin: a.Role == "admin", ActorID: actor})
}
