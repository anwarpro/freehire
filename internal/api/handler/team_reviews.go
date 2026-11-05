package handler

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/strelov1/freehire/internal/api/ratelimit"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
	"github.com/strelov1/freehire/internal/platform/db"
)

type teamReviewHandlers struct{ service *teamreview.Service }

func newTeamReviewHandlers(pool *pgxpool.Pool, q *db.Queries) *teamReviewHandlers {
	return &teamReviewHandlers{service: teamreview.New(pool, q, authHasher{})}
}
func (h *teamReviewHandlers) register(api fiber.Router, mw middleware) {
	group := api.Group("/team", mw.cookie, func(c *fiber.Ctx) error { c.Set("Cache-Control", "private, no-store"); return c.Next() })
	group.Get("/me", h.Me)
	group.Get("/curriculum", h.curriculumSettings)
	group.Put("/curriculum", h.saveCurriculumSettings)
	group.Get("/curriculum/history", h.curriculumConfigHistory)
	group.Get("/curriculum/jobs", h.curriculumJobs)
	group.Get("/curriculum/jobs/:slug", h.curriculumJob)
	group.Put("/curriculum/jobs/:slug", h.reviewCurriculum)
	group.Get("/curriculum/jobs/:slug/history", h.curriculumHistory)
	group.Get("/members", h.Members)
	group.Post("/members", ratelimit.Middleware(mw.throttler, ratelimit.KeyByUserOrIP("team-create"), 10, time.Minute), h.CreateMember)
	group.Patch("/members/:id", h.UpdateMember)
	group.Get("/members/:id/history", h.MemberHistory)
	group.Get("/student-followups", h.studentFollowups)
	group.Get("/jobs/:slug/students", h.studentFollowups)
	group.Post("/jobs/:slug/students", h.createStudentFollowup)
	group.Get("/jobs/:slug/students/:id", h.studentFollowup)
	group.Put("/jobs/:slug/students/:id", h.updateStudentFollowup)
	group.Post("/jobs/:slug/students/:id/contacts", h.recordStudentContact)
	group.Get("/jobs/:slug/students/:id/history", h.studentFollowupHistory)
	group.Get("/reviews", h.List)
	group.Get("/jobs/:slug", h.Get)
	group.Put("/jobs/:slug", h.Save)
	group.Get("/jobs/:slug/history", h.History)
}
func teamError(err error) error {
	switch {
	case errors.Is(err, teamreview.ErrForbidden):
		return fiber.NewError(403, err.Error())
	case errors.Is(err, teamreview.ErrConflict), errors.Is(err, teamreview.ErrExists):
		return fiber.NewError(409, err.Error())
	case errors.Is(err, teamreview.ErrInvalid):
		return fiber.NewError(400, err.Error())
	default:
		return err
	}
}
func teamBody(c *fiber.Ctx, out any) error {
	if len(c.Body()) > 65536 {
		return fiber.NewError(413, "Team request is too large")
	}
	if err := c.BodyParser(out); err != nil {
		return fiber.NewError(400, "Invalid team request")
	}
	return nil
}
func teamMemberID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fiber.NewError(400, "Invalid team member")
	}
	return id, nil
}
func (h *teamReviewHandlers) Me(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.Access(c.Context(), actor)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) Members(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.Members(c.Context(), actor)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) CreateMember(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	var in teamreview.MemberInput
	if err = teamBody(c, &in); err != nil {
		return err
	}
	id, err := h.service.CreateMember(c.Context(), actor, in)
	if err != nil {
		return teamError(err)
	}
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"id": id}})
}
func (h *teamReviewHandlers) UpdateMember(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := teamMemberID(c)
	if err != nil {
		return err
	}
	var in teamreview.MemberUpdate
	if err = teamBody(c, &in); err != nil {
		return err
	}
	if err = h.service.UpdateMember(c.Context(), actor, id, in); err != nil {
		return teamError(err)
	}
	return c.SendStatus(204)
}
func (h *teamReviewHandlers) MemberHistory(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := teamMemberID(c)
	if err != nil {
		return err
	}
	out, err := h.service.MemberHistory(c.Context(), actor, id)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) List(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	_, offset := pageParamsBounded(c, 50, 50)
	if offset > 100000 {
		return fiber.NewError(400, "Invalid queue offset")
	}
	if raw := c.Query("overdue"); raw != "" && raw != "true" && raw != "false" {
		return fiber.NewError(400, "Invalid overdue filter")
	}
	out, err := h.service.List(c.Context(), actor, teamreview.QueueInput{Scope: c.Query("scope"), Status: c.Query("status"), Overdue: c.Query("overdue") == "true", Offset: int32(offset)})
	if err != nil {
		return teamError(err)
	}
	return c.JSON(out)
}
func (h *teamReviewHandlers) Get(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.Get(c.Context(), actor, c.Params("slug"))
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) Save(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	var in teamreview.ReviewInput
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.Save(c.Context(), actor, c.Params("slug"), in)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) History(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.History(c.Context(), actor, c.Params("slug"))
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
