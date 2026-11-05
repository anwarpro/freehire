package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/ingest/teamreview"
)

func (h *teamReviewHandlers) curriculumSettings(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.CurriculumSettings(c.Context(), actor)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) saveCurriculumSettings(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	var in teamreview.CurriculumConfigInput
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.SaveCurriculumSettings(c.Context(), actor, in)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) curriculumConfigHistory(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.CurriculumConfigHistory(c.Context(), actor)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) curriculumJobs(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	_, offset := pageParamsBounded(c, 50, 50)
	out, err := h.service.CurriculumJobs(c.Context(), actor, teamreview.CurriculumQueueInput{Track: c.Query("track"), Source: c.Query("source"), State: c.Query("state"), Review: c.Query("review"), Offset: int32(offset)})
	if err != nil {
		return teamError(err)
	}
	return c.JSON(out)
}
func (h *teamReviewHandlers) curriculumJob(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.CurriculumJob(c.Context(), actor, c.Params("slug"))
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) reviewCurriculum(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	var in teamreview.CurriculumReviewInput
	if err = teamBody(c, &in); err != nil {
		return err
	}
	out, err := h.service.ReviewCurriculum(c.Context(), actor, c.Params("slug"), in)
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
func (h *teamReviewHandlers) curriculumHistory(c *fiber.Ctx) error {
	actor, err := requireUserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.CurriculumHistory(c.Context(), actor, c.Params("slug"))
	if err != nil {
		return teamError(err)
	}
	return c.JSON(fiber.Map{"data": out})
}
