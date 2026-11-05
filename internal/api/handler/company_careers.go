package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/strelov1/freehire/internal/ingest/boardcatalog"
	"github.com/strelov1/freehire/internal/ingest/companycareer"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/pgconv"
)

type companyCareerHandlers struct {
	queries *db.Queries
	service *companycareer.Service
}

func newCompanyCareerHandlers(q *db.Queries) *companyCareerHandlers {
	return &companyCareerHandlers{q, companycareer.New(boardcatalog.NewQueriesRepository(q))}
}
func (h *companyCareerHandlers) register(api fiber.Router, mw middleware) {
	api.Get("/moderation/career-pages", mw.key, mw.moderator, h.List)
	api.Post("/moderation/career-pages/preview", mw.key, mw.moderator, mw.outboundFetch, h.Preview)
	api.Post("/moderation/career-pages", mw.key, mw.moderator, mw.outboundFetch, h.Enable)
	api.Delete("/moderation/career-pages/:id", mw.key, mw.moderator, h.Retire)
}
func (h *companyCareerHandlers) List(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.queries.ListCompanyCareerPages(ctx)
	if err != nil {
		return err
	}
	if len(rows) > 200 {
		return fiber.NewError(503, "Career-source list exceeds 200 rows; contact the operator")
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		// Never expose the raw upstream error: it may carry credentials or response data.
		state := "awaiting_first_collection"
		if r.Status == "retired" {
			state = "paused"
		} else if r.ConsecutiveFailures.Int32 > 0 {
			state = "collection_failed"
		} else if r.LastSuccessAt.Valid {
			state = "healthy"
		}
		out = append(out, fiber.Map{"id": r.ID, "config": json.RawMessage(r.CareerConfig), "company": r.Company, "url": pgconv.TextString(r.URL), "provider": r.Provider, "board": r.Board, "status": r.Status, "state": state, "last_run_at": pgconv.TimePtr(r.LastRunAt), "last_success_at": pgconv.TimePtr(r.LastSuccessAt), "last_ingested_count": r.LastIngestedCount, "failures": r.ConsecutiveFailures, "cooldown_until": pgconv.TimePtr(r.CooldownUntil), "quiet_until": pgconv.TimePtr(r.QuietUntil)})
	}
	return c.JSON(fiber.Map{"data": out, "shortlist": companycareer.Shortlist()})
}
func careerInput(c *fiber.Ctx) (companycareer.Input, error) {
	var in companycareer.Input
	if len(c.Body()) > 8192 {
		return in, fiber.NewError(413, "Career-page configuration is too large")
	}
	if err := c.BodyParser(&in); err != nil {
		return in, fiber.NewError(400, "Invalid career-page configuration")
	}
	return in, nil
}
func (h *companyCareerHandlers) Preview(c *fiber.Ctx) error {
	in, err := careerInput(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	preview, err := h.service.Preview(ctx, in)
	if err != nil {
		return fiber.NewError(422, "Could not verify this career page. Check the URL/selectors or use manual job intake.")
	}
	return c.JSON(fiber.Map{"data": preview})
}
func (h *companyCareerHandlers) Enable(c *fiber.Ctx) error {
	in, err := careerInput(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	board, preview, err := h.service.Enable(ctx, in)
	if errors.Is(err, boardcatalog.ErrDuplicateBoard) {
		return fiber.NewError(409, "This career source is already enabled")
	}
	if err != nil {
		return fiber.NewError(422, "Could not enable this career page. It must pass verification before collection is scheduled.")
	}
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"id": board.ID, "preview": preview}})
}
func (h *companyCareerHandlers) Retire(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.NewError(400, "Invalid career source")
	}
	row, err := h.queries.GetCompanyCareerPage(c.Context(), int64(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = boardcatalog.NewQueriesRepository(h.queries).Retire(c.Context(), row.Provider, row.Board, row.Region)
	if err != nil {
		return err
	}
	return c.SendStatus(204)
}
