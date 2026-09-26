package compute

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const marketplaceDatabaseFilename = "compute-marketplace.db"

func openMarketplaceDB(
	dataDir string,
) (*sql.DB, error) {

	if err := os.MkdirAll(
		dataDir,
		0755,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot create compute marketplace data directory: %w",
			err,
		)
	}

	path := filepath.Join(
		dataDir,
		marketplaceDatabaseFilename,
	)

	db, err := sql.Open(
		"sqlite",
		path,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot open compute marketplace database: %w",
			err,
		)
	}

	// Keep SQLite access serialized for the local Prism node.
	db.SetMaxOpenConns(1)

	if err := configureMarketplaceDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := createMarketplaceSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func configureMarketplaceDB(
	db *sql.DB,
) error {

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf(
				"cannot configure compute marketplace database: %w",
				err,
			)
		}
	}

	return nil
}

func createMarketplaceSchema(
	db *sql.DB,
) error {

	const schema = `
CREATE TABLE IF NOT EXISTS compute_meta (
key TEXT PRIMARY KEY,
value TEXT NOT NULL
);

INSERT INTO compute_meta (key, value)
VALUES ('schema_version', '1')
ON CONFLICT(key) DO NOTHING;

CREATE TABLE IF NOT EXISTS compute_jobs (
id TEXT PRIMARY KEY,
task_id TEXT NOT NULL,
task_json TEXT NOT NULL,

requester TEXT NOT NULL,

reward TEXT NOT NULL,
work_units TEXT NOT NULL,
nonce TEXT NOT NULL,

status TEXT NOT NULL
CHECK (
status IN (
'OPEN',
'CLAIMED',
'VERIFIED'
)
),

worker TEXT NOT NULL DEFAULT '',
proof_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_compute_jobs_status
ON compute_jobs(status);

CREATE INDEX IF NOT EXISTS idx_compute_jobs_requester
ON compute_jobs(requester);

CREATE INDEX IF NOT EXISTS idx_compute_jobs_worker
ON compute_jobs(worker);

CREATE INDEX IF NOT EXISTS idx_compute_jobs_proof_id
ON compute_jobs(proof_id);
`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf(
			"cannot initialize compute marketplace schema: %w",
			err,
		)
	}

	return nil
}
