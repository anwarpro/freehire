package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/strelov1/freehire/internal/job/jobview"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/pgconv"
)

// The archive intentionally has a smaller filter vocabulary than active Meili search.
// Unsupported filters are refused, so an active-only query cannot silently widen here.
var closedJobParams = []string{"limit", "before", "source", "source_exclude", "skills", "skills_exclude", "skills_mode", "countries", "countries_exclude"}

type closedJobCursor struct {
	ClosedAt time.Time `json:"at"`
	Slug     string    `json:"slug"`
}

func parseClosedJobs(values url.Values) (db.ListClosedCatalogueJobsParams, error) {
	p := db.ListClosedCatalogueJobsParams{}
	for key := range values {
		if !slices.Contains(closedJobParams, key) {
			return p, fiber.NewError(fiber.StatusBadRequest, "unsupported closed-job filter: "+key)
		}
	}
	fields := []struct {
		key    string
		target *[]string
	}{
		{"source", &p.Sources}, {"source_exclude", &p.ExcludedSources},
		{"skills", &p.Skills}, {"skills_exclude", &p.ExcludedSkills},
		{"countries", &p.Countries}, {"countries_exclude", &p.ExcludedCountries},
	}
	for _, field := range fields {
		raw := strings.Join(values[field.key], ",")
		if len(raw) > 1024 {
			return p, fiber.NewError(fiber.StatusBadRequest, "filter is too long: "+field.key)
		}
		*field.target = []string{}
		for _, value := range strings.Split(raw, ",") {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" || slices.Contains(*field.target, value) {
				continue
			}
			if len(value) > 64 || len(*field.target) >= 20 {
				return p, fiber.NewError(fiber.StatusBadRequest, "too many or invalid filter values: "+field.key)
			}
			if strings.HasPrefix(field.key, "countries") && (len(value) != 2 || value[0] < 'a' || value[0] > 'z' || value[1] < 'a' || value[1] > 'z') {
				return p, fiber.NewError(fiber.StatusBadRequest, "countries must use two-letter codes, such as bd")
			}
			*field.target = append(*field.target, value)
		}
	}
	mode := values.Get("skills_mode")
	if mode != "" && mode != "or" && mode != "and" {
		return p, fiber.NewError(fiber.StatusBadRequest, "skills_mode must be or or and")
	}
	p.AllSkills = mode == "and"
	if raw := values.Get("before"); raw != "" {
		if len(raw) > 2048 {
			return p, fiber.NewError(fiber.StatusBadRequest, "invalid archive cursor")
		}
		b, err := base64.RawURLEncoding.DecodeString(raw)
		var cursor closedJobCursor
		if err != nil || json.Unmarshal(b, &cursor) != nil || cursor.ClosedAt.IsZero() || cursor.Slug == "" || len(cursor.Slug) > 1024 {
			return p, fiber.NewError(fiber.StatusBadRequest, "invalid archive cursor")
		}
		p.BeforeClosedAt = pgconv.Timestamptz(&cursor.ClosedAt)
		p.BeforeSlug = cursor.Slug
	}
	return p, nil
}

// ClosedJobs uses the existing public-read budget, a five-second query deadline,
// indexed keyset pagination, and no catalogue-wide count. A cursor contains only
// fields already public on the job; internal numeric IDs never leave this endpoint.
func (h *jobsHandlers) ClosedJobs(c *fiber.Ctx) error {
	p, err := parseClosedJobs(queryValues(c))
	if err != nil {
		return err
	}
	limit, _, err := pageParams(c)
	if err != nil {
		return err
	}
	p.PageLimit = int32(limit + 1)
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.queries.ListClosedCatalogueJobs(ctx, p)
	if err != nil {
		return err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	views, err := jobview.FromRows(rows)
	if err != nil {
		return err
	}
	next := ""
	if hasMore {
		last := rows[len(rows)-1]
		b, err := json.Marshal(closedJobCursor{ClosedAt: last.ClosedAt.Time, Slug: last.PublicSlug})
		if err != nil {
			return err
		}
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	return c.JSON(fiber.Map{"data": views, "meta": fiber.Map{"limit": limit, "has_more": hasMore, "next_cursor": next}})
}
