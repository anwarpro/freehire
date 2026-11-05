package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
	"strconv"
)

func studentFollowupID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fiber.NewError(400, "Invalid student follow-up")
	}
	return id, nil
}
func (h *teamReviewHandlers) studentFollowups(c *fiber.Ctx) error {
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
	out, err := h.service.StudentFollowups(c.Context(), actor, teamreview.StudentFollowupFilter{Slug: c.Params("slug"), Status: c.Query("status"), Query: c.Query("q"), Overdue: c.Query("overdue") == "true", Offset: int32(offset)})
	if err != nil {
		return teamError(err)
	}
	return c.JSON(out)
}
func (h *teamReviewHandlers) createStudentFollowup(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	var in teamreview.StudentFollowupCreate
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.CreateStudentFollowup(c.Context(), actor, c.Params("slug"), in)
	if err != nil {
		return teamError(err)
	}
	return c.Status(201).JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) updateStudentFollowup(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := studentFollowupID(c)
	if err != nil {
		return err
	}
	var in teamreview.StudentFollowupUpdate
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.UpdateStudentFollowup(c.Context(), actor, c.Params("slug"), id, in)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) recordStudentContact(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := studentFollowupID(c)
	if err != nil {
		return err
	}
	var in teamreview.StudentContactInput
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.RecordStudentContact(c.Context(), actor, c.Params("slug"), id, in)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) studentFollowupHistory(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := studentFollowupID(c)
	if err != nil {
		return err
	}
	out, err := h.service.StudentFollowupHistory(c.Context(), actor, c.Params("slug"), id)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}

func (h *teamReviewHandlers) studentFollowup(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	id, err := studentFollowupID(c)
	if err != nil {
		return err
	}
	out, err := h.service.StudentFollowup(c.Context(), actor, c.Params("slug"), id)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
