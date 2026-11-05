// Package companycareer manages curator-verified company career pages through the
// existing board catalogue. Collection, identity, retries and closure stay in ingest.
package companycareer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/strelov1/freehire/internal/ingest/atsboard"
	"github.com/strelov1/freehire/internal/ingest/boardcatalog"
	"github.com/strelov1/freehire/internal/ingest/boardresolve"
	"github.com/strelov1/freehire/internal/ingest/sources"
)

// Input is a company page to preview and optionally enable. Config is only needed
// for custom HTML; recognized ATS URLs keep their existing dedicated adapter.
type Input struct {
	Company string                    `json:"company"`
	URL     string                    `json:"url"`
	Config  *sources.CareerPageConfig `json:"config,omitempty"`
}
type Preview struct {
	Provider string               `json:"provider"`
	Board    string               `json:"board"`
	Active   int                  `json:"active"`
	Closed   int                  `json:"closed"`
	Samples  []Sample             `json:"samples"`
	Entry    sources.CompanyEntry `json:"-"`
}
type Sample struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Closed bool   `json:"closed"`
}
type Service struct {
	Registry map[string]sources.Source
	Repo     boardcatalog.Repository
	Resolver *boardresolve.Resolver
}

func New(repo boardcatalog.Repository) *Service {
	return &Service{Registry: sources.All(sources.NewClient()), Repo: repo, Resolver: boardresolve.New()}
}

func (s *Service) Preview(ctx context.Context, in Input) (Preview, error) {
	in.Company = strings.TrimSpace(in.Company)
	if in.Company == "" || len(in.Company) > 200 {
		return Preview{}, fmt.Errorf("company name is required (maximum 200 characters)")
	}
	canonical, err := sources.CareerPageURL(in.URL)
	if err != nil {
		return Preview{}, err
	}
	e := sources.CompanyEntry{Company: in.Company, Provider: "careerpage", Board: canonical, CareerPage: in.Config}
	for _, seed := range Shortlist() {
		if seed.URL == canonical {
			if seed.Fallback != "" {
				if in.Config == nil {
					return Preview{}, fmt.Errorf("%s", seed.Fallback)
				}
				break
			}
			e.Provider = seed.Provider
			e.Board = seed.Board
			if in.Config == nil {
				e.CareerPage = seed.Config
			}
			break
		}
	}
	if e.Provider == "careerpage" && e.CareerPage == nil {
		if provider, board, _, ok := atsboard.Recognize(canonical); ok {
			e.Provider = provider
			e.Board = board
		} else if s.Resolver != nil {
			if provider, board, _, ok := s.Resolver.Resolve(ctx, canonical); ok {
				e.Provider = provider
				e.Board = board
			}
		}
	}
	if err := (sources.Config{Provider: e.Provider, Sources: []sources.CompanyEntry{e}}).Validate(s.Registry); err != nil {
		return Preview{}, err
	}
	src, ok := s.Registry[e.Provider]
	if !ok {
		return Preview{}, fmt.Errorf("source adapter is unavailable")
	}
	jobs, err := src.Fetch(ctx, e)
	if err != nil {
		return Preview{}, fmt.Errorf("could not verify collection: %w", err)
	}
	out := Preview{Provider: e.Provider, Board: e.Board, Entry: e, Samples: []Sample{}}
	for _, j := range jobs {
		if j.Unreadable {
			return Preview{}, fmt.Errorf("a listed job could not be read; collection was not enabled")
		}
		if !j.Removed && (j.Title == "" || j.Description == "" || j.URL == "") {
			return Preview{}, fmt.Errorf("adapter returned an incomplete job")
		}
		if j.Removed {
			out.Closed++
		} else {
			out.Active++
		}
		if len(out.Samples) < 5 {
			out.Samples = append(out.Samples, Sample{j.Title, j.URL, j.Removed})
		}
	}
	return out, nil
}
func (s *Service) Enable(ctx context.Context, in Input) (boardcatalog.Board, Preview, error) {
	preview, err := s.Preview(ctx, in)
	if err != nil {
		return boardcatalog.Board{}, Preview{}, err
	}
	if s.Repo == nil {
		return boardcatalog.Board{}, Preview{}, errors.New("catalogue unavailable")
	}
	e := preview.Entry
	canonical, _ := sources.CareerPageURL(in.URL)
	row, err := boardcatalog.NewInserter(s.Repo, s.Registry).Insert(ctx, boardcatalog.InsertInput{Company: strings.TrimSpace(in.Company), URL: canonical, Provider: e.Provider, Board: e.Board, Region: e.Region, CareerPage: e.CareerPage, Surface: "company-career"}, boardcatalog.StatusPending)
	return row, preview, err
}
