//go:build pgdb
// +build pgdb

package internal

import (
	"context"
	"testing"
	"time"

	brpgdb "github.com/companyzero/bisonrelay/server/internal/pgdb"
)

// TestPGDB tests the serverdb implementation that is backed by a postgres db.
//
// Running this test requires having a local database accessible via the
// network address 127.0.0.1:5432 named 'brdatasim', a role
// named 'brdatasim' accessible with password 'brdatasim' with the appropriate
// permissions and using brbulksim and brindexsim tablespaces.
//
// See the README doc in the pgdb dir for instructions.
func TestPGDB(t *testing.T) {
	opts := []brpgdb.Option{
		brpgdb.WithRole("brdatasim"),
		brpgdb.WithPassphrase("brdatasim"),
		brpgdb.WithDBName("brdatasim"),
		brpgdb.WithBulkDataTablespace("brbulksim"),
		brpgdb.WithIndexTablespace("brindexsim"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := brpgdb.Open(ctx, opts...)
	if err != nil {
		t.Fatalf("Unable to open db: %v", err)
	}
	defer db.Close()

	testServerDBInterface(t, db)
}
