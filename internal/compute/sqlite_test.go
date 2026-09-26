package compute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMarketplaceDB(
	t *testing.T,
) {

	dataDir := t.TempDir()

	db, err := openMarketplaceDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	path := filepath.Join(
		dataDir,
		marketplaceDatabaseFilename,
	)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf(
			"compute marketplace database was not created: %v",
			err,
		)
	}

	var version string

	if err := db.QueryRow(`
SELECT value
FROM compute_meta
WHERE key = 'schema_version'
`).Scan(&version); err != nil {
		t.Fatal(err)
	}

	if version != "1" {
		t.Fatalf(
			"unexpected schema version: %s",
			version,
		)
	}
}
