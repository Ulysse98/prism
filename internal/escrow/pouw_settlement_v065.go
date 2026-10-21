package escrow

import (
	"fmt"

	"prism/internal/compute"
	"prism/internal/usefulwork"
)

// VerifyEscrowClaimV065 checks the worker's signed, network-bound claim and
// advances a copy of canonical escrow state. Consensus callers must obtain
// current and height by replaying blocks; these inputs must never be taken
// from unauthenticated API payloads.
func VerifyEscrowClaimV065(current Escrow, claim compute.ClaimAuthorization, height uint64) (Escrow, error) {
	if err := current.Validate(); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow state: %w", err)
	}
	if err := compute.VerifyClaimAuthorization(
		claim, current.Funding.JobID, current.Funding.ChainID, current.Funding.GenesisHash,
	); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow claim authorization: %w", err)
	}
	next := current
	if err := next.Claim(claim.Worker, height); err != nil {
		return Escrow{}, err
	}
	return next, nil
}

// VerifyEscrowReleaseV065 checks a signed, job-scoped PoUW proof and its
// actual task/result before changing the escrow lifecycle. It NEVER credits
// balances: consensus replay must atomically debit locked requester funds,
// credit the worker and update the escrow index exactly once.
func VerifyEscrowReleaseV065(current Escrow, proof usefulwork.Proof, height uint64) (Escrow, error) {
	if err := current.Validate(); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow state: %w", err)
	}
	if current.Status != Claimed {
		return Escrow{}, fmt.Errorf("escrow has no active claim")
	}
	if err := usefulwork.VerifyComputeProofContext(
		proof, current.Funding.JobID, current.Funding.ChainID, current.Funding.GenesisHash,
	); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow proof context: %w", err)
	}
	if proof.Task.ID != current.Funding.TaskID {
		return Escrow{}, fmt.Errorf("escrow proof task differs from funded task")
	}
	if proof.Worker != current.Worker {
		return Escrow{}, fmt.Errorf("escrow proof worker differs from claimant")
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow useful-work proof: %w", err)
	}
	next := current
	ref := ProofReference{
		ID: proof.ID, ChainID: proof.ChainID, GenesisHash: proof.GenesisHash,
		JobID: proof.JobID, Worker: proof.Worker,
	}
	if err := next.Release(ref, height); err != nil {
		return Escrow{}, err
	}
	return next, nil
}

// VerifyEscrowRefundV065 authorizes a *permissionless* expiry refund of an
// existing escrow. Any validator may include the refund at/after expiry; it
// always returns funds to the original requester. Only consensus replay may
// make the corresponding balance/lock changes.
func VerifyEscrowRefundV065(current Escrow, height uint64) (Escrow, error) {
	if err := current.Validate(); err != nil {
		return Escrow{}, fmt.Errorf("invalid escrow state: %w", err)
	}
	next := current
	if err := next.Refund(height); err != nil {
		return Escrow{}, err
	}
	return next, nil
}
