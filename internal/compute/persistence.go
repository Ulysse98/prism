package compute

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"prism/internal/usefulwork"
)

const marketplaceFilename = "compute-jobs.json"

type marketplaceSnapshot struct {
	Version uint64 `json:"version"`
	Jobs    []Job  `json:"jobs"`
}

type sqlExecer interface {
	Exec(
		query string,
		args ...any,
	) (sql.Result, error)
}

func NewPersistentMarketplace(
	dataDir string,
) (*Marketplace, error) {

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, fmt.Errorf(
			"compute marketplace data directory cannot be empty",
		)
	}

	db, err := openMarketplaceDB(dataDir)
	if err != nil {
		return nil, err
	}

	market := &Marketplace{
		jobs:    make(map[string]Job),
		dataDir: dataDir,
		db:      db,
	}

	if err := market.load(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return market, nil
}

func (market *Marketplace) Close() error {
	if market == nil || market.db == nil {
		return nil
	}

	return market.db.Close()
}

func (market *Marketplace) load() error {
	if market.db == nil {
		return fmt.Errorf(
			"compute marketplace database is not open",
		)
	}

	if err := market.importLegacyJSONIfNeeded(); err != nil {
		return err
	}

	rows, err := market.db.Query(`
SELECT
id,
task_id,
task_json,
requester,
reward,
work_units,
nonce,
status,
worker,
proof_id
FROM compute_jobs
ORDER BY id
`)
	if err != nil {
		return fmt.Errorf(
			"cannot load compute marketplace database: %w",
			err,
		)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			jobID         string
			taskID        string
			taskJSON      string
			requester     string
			rewardText    string
			workUnitsText string
			nonceText     string
			statusText    string
			worker        string
			proofID       string
		)

		if err := rows.Scan(
			&jobID,
			&taskID,
			&taskJSON,
			&requester,
			&rewardText,
			&workUnitsText,
			&nonceText,
			&statusText,
			&worker,
			&proofID,
		); err != nil {
			return fmt.Errorf(
				"cannot scan compute marketplace job: %w",
				err,
			)
		}

		var task usefulwork.Task

		if err := json.Unmarshal(
			[]byte(taskJSON),
			&task,
		); err != nil {
			return fmt.Errorf(
				"cannot decode compute task %s: %w",
				jobID,
				err,
			)
		}

		if task.ID != taskID {
			return fmt.Errorf(
				"compute job %s task ID mismatch",
				jobID,
			)
		}

		reward, err := parsePersistedUint64(
			"reward",
			rewardText,
		)
		if err != nil {
			return err
		}

		workUnits, err := parsePersistedUint64(
			"work units",
			workUnitsText,
		)
		if err != nil {
			return err
		}

		nonce, err := parsePersistedUint64(
			"nonce",
			nonceText,
		)
		if err != nil {
			return err
		}

		job := Job{
			ID:        jobID,
			Task:      task,
			Requester: requester,
			Reward:    reward,
			WorkUnits: workUnits,
			Nonce:     nonce,
			Status:    JobStatus(statusText),
			Worker:    worker,
			ProofID:   proofID,
		}

		if err := validatePersistedJob(job); err != nil {
			return fmt.Errorf(
				"invalid persisted compute job %s: %w",
				job.ID,
				err,
			)
		}

		if _, exists := market.jobs[job.ID]; exists {
			return fmt.Errorf(
				"duplicate persisted compute job: %s",
				job.ID,
			)
		}

		market.jobs[job.ID] = job
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf(
			"cannot iterate compute marketplace database: %w",
			err,
		)
	}

	return nil
}

func (
	market *Marketplace,
) importLegacyJSONIfNeeded() error {

	var count uint64

	if err := market.db.QueryRow(`
SELECT COUNT(*)
FROM compute_jobs
`).Scan(&count); err != nil {
		return fmt.Errorf(
			"cannot inspect compute marketplace database: %w",
			err,
		)
	}

	// SQLite already owns the marketplace state.
	if count != 0 {
		return nil
	}

	path := filepath.Join(
		market.dataDir,
		marketplaceFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf(
			"cannot load legacy compute marketplace: %w",
			err,
		)
	}

	var snapshot marketplaceSnapshot

	if err := json.Unmarshal(
		data,
		&snapshot,
	); err != nil {
		return fmt.Errorf(
			"cannot decode legacy compute marketplace: %w",
			err,
		)
	}

	if snapshot.Version != 1 {
		return fmt.Errorf(
			"unsupported legacy compute marketplace version: %d",
			snapshot.Version,
		)
	}

	seen := make(map[string]struct{})

	for _, job := range snapshot.Jobs {
		if err := validatePersistedJob(job); err != nil {
			return fmt.Errorf(
				"invalid legacy compute job %s: %w",
				job.ID,
				err,
			)
		}

		if _, exists := seen[job.ID]; exists {
			return fmt.Errorf(
				"duplicate legacy compute job: %s",
				job.ID,
			)
		}

		seen[job.ID] = struct{}{}
	}

	tx, err := market.db.Begin()
	if err != nil {
		return fmt.Errorf(
			"cannot start legacy compute marketplace migration: %w",
			err,
		)
	}

	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, job := range snapshot.Jobs {
		if err := insertMarketplaceJob(
			tx,
			job,
		); err != nil {
			return fmt.Errorf(
				"cannot migrate legacy compute job %s: %w",
				job.ID,
				err,
			)
		}
	}

	if _, err := tx.Exec(`
INSERT INTO compute_meta (
key,
value
)
VALUES (
'legacy_json_imported',
'1'
)
ON CONFLICT(key)
DO UPDATE SET value = excluded.value
`); err != nil {
		return fmt.Errorf(
			"cannot record compute marketplace migration: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"cannot commit legacy compute marketplace migration: %w",
			err,
		)
	}

	committed = true

	return nil
}

func (market *Marketplace) persistLocked() error {
	// In-memory marketplaces created with NewMarketplace
	// intentionally have no persistent database.
	if market.db == nil {
		return nil
	}

	tx, err := market.db.Begin()
	if err != nil {
		return fmt.Errorf(
			"cannot start compute marketplace transaction: %w",
			err,
		)
	}

	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
DELETE FROM compute_jobs
`); err != nil {
		return fmt.Errorf(
			"cannot clear compute marketplace jobs: %w",
			err,
		)
	}

	for _, job := range market.listLocked() {
		if err := validatePersistedJob(job); err != nil {
			return fmt.Errorf(
				"invalid compute job %s: %w",
				job.ID,
				err,
			)
		}

		if err := insertMarketplaceJob(
			tx,
			job,
		); err != nil {
			return fmt.Errorf(
				"cannot persist compute job %s: %w",
				job.ID,
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"cannot commit compute marketplace transaction: %w",
			err,
		)
	}

	committed = true

	return nil
}

func insertMarketplaceJob(
	exec sqlExecer,
	job Job,
) error {

	taskJSON, err := json.Marshal(job.Task)
	if err != nil {
		return fmt.Errorf(
			"cannot encode compute task: %w",
			err,
		)
	}

	_, err = exec.Exec(`
INSERT INTO compute_jobs (
id,
task_id,
task_json,
requester,
reward,
work_units,
nonce,
status,
worker,
proof_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		job.ID,
		job.Task.ID,
		string(taskJSON),
		job.Requester,
		strconv.FormatUint(job.Reward, 10),
		strconv.FormatUint(job.WorkUnits, 10),
		strconv.FormatUint(job.Nonce, 10),
		string(job.Status),
		job.Worker,
		job.ProofID,
	)
	if err != nil {
		return err
	}

	return nil
}

func parsePersistedUint64(
	field string,
	value string,
) (uint64, error) {

	parsed, err := strconv.ParseUint(
		value,
		10,
		64,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid persisted compute %s %q: %w",
			field,
			value,
			err,
		)
	}

	return parsed, nil
}

func validatePersistedJob(
	job Job,
) error {

	if err := usefulwork.ValidateTask(
		job.Task,
	); err != nil {
		return err
	}

	if strings.TrimSpace(job.Requester) == "" {
		return fmt.Errorf(
			"requester cannot be empty",
		)
	}

	if job.Reward == 0 {
		return fmt.Errorf(
			"reward must be greater than zero",
		)
	}

	workUnits, err := usefulwork.WorkUnits(
		job.Task,
	)
	if err != nil {
		return err
	}

	if job.WorkUnits != workUnits {
		return fmt.Errorf(
			"invalid work units",
		)
	}

	if job.ID != CalculateJobID(job) {
		return fmt.Errorf(
			"invalid compute job ID",
		)
	}

	switch job.Status {
	case JobStatusOpen:
		if job.Worker != "" ||
			job.ProofID != "" {

			return fmt.Errorf(
				"OPEN job contains completion state",
			)
		}

	case JobStatusClaimed:
		if strings.TrimSpace(job.Worker) == "" {
			return fmt.Errorf(
				"CLAIMED job has no worker",
			)
		}

		if job.ProofID != "" {
			return fmt.Errorf(
				"CLAIMED job contains proof ID",
			)
		}

	case JobStatusVerified:
		if strings.TrimSpace(job.Worker) == "" {
			return fmt.Errorf(
				"VERIFIED job has no worker",
			)
		}

		if strings.TrimSpace(job.ProofID) == "" {
			return fmt.Errorf(
				"VERIFIED job has no proof ID",
			)
		}

	default:
		return fmt.Errorf(
			"invalid compute job status: %s",
			job.Status,
		)
	}

	return nil
}
