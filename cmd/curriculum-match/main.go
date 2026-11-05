// Command curriculum-match updates curriculum suggestions without model calls.
package main

import (
	"context"
	"errors"
	"github.com/strelov1/freehire/internal/job/curriculum"
	"github.com/strelov1/freehire/internal/platform/worker"
	"log"
	"time"
)

func main() { worker.Main(run) }
func run() int {
	ctx, _, pool, cleanup, err := worker.Bootstrap(context.Background())
	if err != nil {
		log.Print(err)
		return 1
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	n, err := curriculum.Drain(ctx, pool, 2000)
	if recordErr := curriculum.RecordRun(pool, n, err); recordErr != nil {
		log.Printf("curriculum: record progress: %v", recordErr)
		return 1
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		log.Printf("curriculum: processed=%d failed: %v", n, err)
		return 1
	}
	log.Printf("curriculum: processed=%d partial=%t", n, err != nil)
	return 0
}
