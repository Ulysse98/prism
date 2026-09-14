package compute

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"prism/internal/usefulwork"
)

type JobStatus string

const (
	JobStatusOpen     JobStatus = "OPEN"
	JobStatusClaimed  JobStatus = "CLAIMED"
	JobStatusVerified JobStatus = "VERIFIED"
)

type Job struct {
	ID        string          `json:"id"`
	Task      usefulwork.Task `json:"task"`
	Requester string          `json:"requester"`
	Reward    uint64          `json:"reward"`
	WorkUnits uint64          `json:"workUnits"`
	Nonce     uint64          `json:"nonce"`
	Status    JobStatus       `json:"status"`
	Worker    string          `json:"worker,omitempty"`
	ProofID   string          `json:"proofId,omitempty"`
}

func NewJob(
	task usefulwork.Task,
	requester string,
	reward uint64,
	nonce uint64,
) (Job, error) {

	if err := usefulwork.ValidateTask(task); err != nil {
		return Job{}, err
	}

	requester = strings.TrimSpace(requester)
	if requester == "" {
		return Job{}, fmt.Errorf(
			"compute requester cannot be empty",
		)
	}

	if reward == 0 {
		return Job{}, fmt.Errorf(
			"compute reward must be greater than zero",
		)
	}

	workUnits, err := usefulwork.WorkUnits(task)
	if err != nil {
		return Job{}, err
	}

	job := Job{
		Task:      task,
		Requester: requester,
		Reward:    reward,
		WorkUnits: workUnits,
		Nonce:     nonce,
		Status:    JobStatusOpen,
	}

	job.ID = CalculateJobID(job)

	return job, nil
}

func CalculateJobID(job Job) string {
	payload := fmt.Sprintf(
		"%s|%s|%d|%d",
		job.Task.ID,
		job.Requester,
		job.Reward,
		job.Nonce,
	)

	hash := sha256.Sum256([]byte(payload))

	return hex.EncodeToString(hash[:])
}

func (job *Job) Claim(worker string) error {
	if job == nil {
		return fmt.Errorf(
			"compute job cannot be nil",
		)
	}

	worker = strings.TrimSpace(worker)
	if worker == "" {
		return fmt.Errorf(
			"compute worker cannot be empty",
		)
	}

	if job.Status != JobStatusOpen {
		return fmt.Errorf(
			"compute job is not open",
		)
	}

	job.Worker = worker
	job.Status = JobStatusClaimed

	return nil
}

func (job *Job) Complete(
	proof usefulwork.Proof,
) error {

	if job == nil {
		return fmt.Errorf(
			"compute job cannot be nil",
		)
	}

	if job.Status != JobStatusClaimed {
		return fmt.Errorf(
			"compute job is not claimed",
		)
	}

	if proof.Worker != job.Worker {
		return fmt.Errorf(
			"proof worker does not own compute job",
		)
	}

	if proof.Task.ID != job.Task.ID {
		return fmt.Errorf(
			"proof task does not match compute job",
		)
	}

	if err := usefulwork.VerifyProof(proof); err != nil {
		return err
	}

	job.ProofID = proof.ID
	job.Status = JobStatusVerified

	return nil
}
