// Command hero-bootstrap initializes boardless sources through the existing catalog writer.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"slices"

	"github.com/strelov1/freehire/internal/ingest/boardcatalog"
	"github.com/strelov1/freehire/internal/ingest/sources"
	"github.com/strelov1/freehire/internal/platform/db"
	"github.com/strelov1/freehire/internal/platform/worker"
)

type sourceStatus struct {
	Provider         string `json:"provider"`
	NeedsBoard       bool   `json:"needs_company_board"`
	CredentialsReady bool   `json:"credentials_ready"`
}

func main() { worker.Main(run) }

func run() int {
	apply := flag.Bool("apply", false, "initialize available boardless sources")
	flag.Parse()
	taxonomy := sources.Taxonomy()
	available := sources.All(sources.NewClient())
	keyed := sources.BoardKeyedProviders(taxonomy)
	keys := make([]string, 0, len(taxonomy))
	for key := range taxonomy {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if !*apply {
		result := make([]sourceStatus, 0, len(keys))
		for _, key := range keys {
			_, ready := available[key]
			result = append(result, sourceStatus{key, slices.Contains(keyed, key), ready})
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			log.Printf("bootstrap: %v", err)
			return 1
		}
		return 0
	}
	ctx, _, pool, cleanup, err := worker.Bootstrap(context.Background())
	if err != nil {
		log.Printf("bootstrap: %v", err)
		return 1
	}
	defer cleanup()
	writer := boardcatalog.NewInserter(boardcatalog.NewQueriesRepository(db.New(pool)), available)
	added, existing := 0, 0
	for _, key := range keys {
		if _, ok := available[key]; !ok || slices.Contains(keyed, key) {
			continue
		}
		board, err := writer.Insert(ctx, boardcatalog.InsertInput{Provider: key, Company: key, Surface: "curator"}, boardcatalog.StatusPending)
		if errors.Is(err, boardcatalog.ErrDuplicateBoard) {
			existing++
			continue
		}
		if err != nil {
			log.Printf("%s: %v", key, err)
			return 1
		}
		if board.Status == boardcatalog.StatusRejected {
			log.Printf("%s rejected: %s", key, board.RejectedReason)
			return 1
		}
		added++
	}
	log.Printf("boardless catalog: added=%d existing=%d", added, existing)
	return 0
}
