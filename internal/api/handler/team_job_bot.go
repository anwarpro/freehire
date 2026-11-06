package handler

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/ai/assistant"
	"github.com/strelov1/freehire/internal/ai/plan"
	"github.com/strelov1/freehire/internal/platform/db"
)

type teamBotRequestKey struct{}

func isTeamBotRequest(c *fiber.Ctx) bool { ok, _ := c.Locals(teamBotRequestKey{}).(bool); return ok }
func (h *assistantHandlers) teamBotAccess(ctx context.Context, user int64) error {
	if h.team == nil {
		return fiber.NewError(503, "Team job discussion is unavailable")
	}
	a, err := h.team.Access(ctx, user)
	if err != nil {
		return err
	}
	if a.Role != "admin" && a.Role != "reviewer" {
		return fiber.NewError(403, "Active team access is required")
	}
	return nil
}
func (h *assistantHandlers) registerTeamBot(api fiber.Router, mw middleware) {
	group := api.Group("/team/bot", mw.cookie, func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "private, no-store")
		user, err := requireUserID(c)
		if err != nil {
			return err
		}
		if err = h.teamBotAccess(c.UserContext(), user); err != nil {
			return err
		}
		c.Locals(teamBotRequestKey{}, true)
		return c.Next()
	})
	group.Get("/status", h.teamBotStatus)
	group.Get("/sessions", h.listTeamBotSessions)
	group.Post("/sessions", h.createTeamBotSession)
	group.Get("/sessions/:id", h.GetAssistantSession)
	group.Delete("/sessions/:id", h.DeleteAssistantSession)
	group.Post("/sessions/:id/cancel", h.CancelAssistantTurn)
	group.Post("/sessions/:id/messages", mw.paidAI(plan.FeatureAssistant), h.PostAssistantMessage)
	group.Post("/sessions/:id/retry", mw.paidAI(plan.FeatureAssistant), h.PostAssistantRetry)
}
func (h *assistantHandlers) teamBotStatus(c *fiber.Ctx) error {
	user, err := requireUserID(c)
	if err != nil {
		return err
	}
	paid := false
	if h.plans != nil {
		_, err = h.plans.RequirePaid(c.UserContext(), user)
		if err != nil && !errors.Is(err, plan.ErrPaymentRequired) {
			return err
		}
		paid = err == nil
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"model_configured": h.runner != nil, "paid_access": paid, "can_chat": h.runner != nil && paid, "access_policy": "Active team membership and Pro/Ultra entitlement; existing assistant daily allowance applies.", "language": "English", "data_scope": "Public jobs and automatic curriculum suggestions. No student records or private reviewer notes."}})
}
func (h *assistantHandlers) createTeamBotSession(c *fiber.Ctx) error {
	user, err := requireUserID(c)
	if err != nil {
		return err
	}
	if c.Query("preset") != "" || len(c.Body()) > 1024 {
		return fiber.NewError(400, "Team conversations use the job discussion mode")
	}
	sess, err := h.store.CreateSession(c.UserContext(), user, assistant.PresetTeamJobs, nil, nil)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(fiber.Map{"data": sessionView(sess)})
}
func (h *assistantHandlers) listTeamBotSessions(c *fiber.Ctx) error {
	user, err := requireUserID(c)
	if err != nil {
		return err
	}
	_, offset := pageParamsBounded(c, 50, 50)
	if offset > 100000 {
		return fiber.NewError(400, "Invalid conversation offset")
	}
	rows, err := h.queries.ListTeamBotSessions(c.UserContext(), db.ListTeamBotSessionsParams{UserID: user, Offset: int32(offset)})
	if err != nil {
		return err
	}
	more := len(rows) > 50
	if more {
		rows = rows[:50]
	}
	out := make([]sessionResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, sessionResponse{ID: r.ID.String(), Preset: r.Preset, Label: r.Label.String})
	}
	return c.JSON(fiber.Map{"data": out, "meta": fiber.Map{"has_more": more, "offset": offset, "limit": 50}})
}
func (h *assistantHandlers) teamJobLink(slug string) string {
	return strings.TrimRight(h.origin, "/") + "/jobs/" + url.PathEscape(slug)
}
