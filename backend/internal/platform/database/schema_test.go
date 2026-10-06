package database

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ExpectedMigration must equal the newest migration file, otherwise `admin doctor` would bless a stale schema.
func TestExpectedMigrationMatchesFiles(t *testing.T) {
	files, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	latest := 0
	for _, f := range files {
		n, err := strconv.Atoi(strings.SplitN(filepath.Base(f), "_", 2)[0])
		if err != nil {
			t.Fatalf("migration %s has no numeric prefix", f)
		}
		latest = max(latest, n)
	}
	if latest != ExpectedMigration {
		t.Errorf("newest migration file is %04d but ExpectedMigration = %d: bump it in database/schema.go", latest, ExpectedMigration)
	}
	if _, err := os.Stat("../../../migrations"); err != nil {
		t.Fatal(err)
	}
}
