//go:build integration

package main

import (
	"flag"
	"os"
	"testing"

	"github.com/strelov1/freehire/internal/platform/testdb"
)

// A scoped liveness run must work on a host without unrelated crawl credentials.
// Exercise the actual command setup, including its registry guards and queries.
func TestLinkedInRunWithoutOtherProviderCredentials(t *testing.T) {
	pool := testdb.Pool(t)
	t.Setenv("DATABASE_URL", pool.Config().ConnString())
	t.Setenv("SENTRY_DSN", "")
	for _, key := range []string{"ADZUNA_APP_ID", "ADZUNA_APP_KEY", "WHATJOBS_PUBLISHER_IDS", "WHATJOBS_PUBLISHER_ID"} {
		t.Setenv(key, "")
	}
	oldFlags, oldArgs := flag.CommandLine, os.Args
	flag.CommandLine = flag.NewFlagSet("liveness", flag.ContinueOnError)
	os.Args = []string{"liveness", "-source=linkedin"}
	defer func() { flag.CommandLine = oldFlags; os.Args = oldArgs }()
	if code := run(); code != 0 {
		t.Fatalf("LinkedIn liveness without unrelated credentials exited %d", code)
	}
}
