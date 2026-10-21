package escrow

import (
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func settlementFixtureV065(t *testing.T) (Escrow, compute.ClaimAuthorization, usefulwork.Proof) {
	t.Helper()
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	task, err := usefulwork.NewSumSquaresTask([]uint64{3, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	f := Funding{
		ChainID:     "prism-" + strings.Repeat("a", 16),
		GenesisHash: strings.Repeat("b", 64),
		TaskID:      task.ID, JobNonce: 3, FundingTxID: strings.Repeat("c", 64),
		Requester: alice.Address, Amount: 10,
		FundedAtHeight: 10, ExpiresAtHeight: 20,
	}
	f.JobID = CalculateFundedJobID(f.TaskID, f.Requester, f.Amount, f.JobNonce)
	e, err := NewFunded(f)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := compute.SignClaimAuthorization(f.JobID, f.ChainID, f.GenesisHash, bob)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, f.JobID, f.ChainID, f.GenesisHash, bob)
	if err != nil {
		t.Fatal(err)
	}
	return e, claim, proof
}

func TestEscrowV065SignedClaimAndVerifiedRelease(t *testing.T) {
	e, claim, proof := settlementFixtureV065(t)
	claimed, err := VerifyEscrowClaimV065(e, claim, 11)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != Claimed || e.Status != Funded {
		t.Fatal("claim mutated input or has incorrect status")
	}
	released, err := VerifyEscrowReleaseV065(claimed, proof, 12)
	if err != nil {
		t.Fatal(err)
	}
	if released.Status != Released || released.ProofID != proof.ID {
		t.Fatal("release did not bind signed proof")
	}
	if claimed.Status != Claimed {
		t.Fatal("release mutated input")
	}
	if _, err := VerifyEscrowReleaseV065(released, proof, 13); err == nil {
		t.Fatal("double release accepted")
	}
	if _, err := VerifyEscrowRefundV065(released, 20); err == nil {
		t.Fatal("refund after release accepted")
	}
}

func TestEscrowV065ClaimRejectsTamperingAndExpiredHeight(t *testing.T) {
	e, claim, _ := settlementFixtureV065(t)
	bad := claim
	bad.Worker = e.Funding.Requester
	if _, err := VerifyEscrowClaimV065(e, bad, 11); err == nil {
		t.Fatal("foreign worker accepted")
	}
	bad = claim
	bad.Signature = strings.Repeat("0", 128)
	if _, err := VerifyEscrowClaimV065(e, bad, 11); err == nil {
		t.Fatal("bad claim signature accepted")
	}
	if _, err := VerifyEscrowClaimV065(e, claim, 20); err == nil {
		t.Fatal("expired claim accepted")
	}
	if e.Status != Funded {
		t.Fatal("failed claim mutated input")
	}
}

func TestEscrowV065ReleaseRejectsWrongTaskAndInvalidProof(t *testing.T) {
	e, claim, proof := settlementFixtureV065(t)
	claimed, err := VerifyEscrowClaimV065(e, claim, 11)
	if err != nil {
		t.Fatal(err)
	}
	bad := proof
	bad.Result++
	if _, err := VerifyEscrowReleaseV065(claimed, bad, 12); err == nil {
		t.Fatal("tampered result accepted")
	}
	bad = proof
	bad.ChainID = "prism-" + strings.Repeat("d", 16)
	if _, err := VerifyEscrowReleaseV065(claimed, bad, 12); err == nil {
		t.Fatal("foreign network accepted")
	}
	bad = proof
	bad.Task.ID = strings.Repeat("d", 64)
	if _, err := VerifyEscrowReleaseV065(claimed, bad, 12); err == nil {
		t.Fatal("foreign task accepted")
	}
	if _, err := VerifyEscrowReleaseV065(claimed, proof, 20); err == nil {
		t.Fatal("expired release accepted")
	}
	if claimed.Status != Claimed {
		t.Fatal("failed release mutated input")
	}
}

func TestEscrowV065PermissionlessRefundAfterExpiry(t *testing.T) {
	e, claim, proof := settlementFixtureV065(t)
	if _, err := VerifyEscrowRefundV065(e, 19); err == nil {
		t.Fatal("early refund accepted")
	}
	refunded, err := VerifyEscrowRefundV065(e, 20)
	if err != nil {
		t.Fatal(err)
	}
	if refunded.Status != Refunded || refunded.Funding.Requester != e.Funding.Requester {
		t.Fatal("refund recipient mismatch")
	}
	if _, err := VerifyEscrowRefundV065(refunded, 21); err == nil {
		t.Fatal("duplicate refund accepted")
	}
	claimed, err := VerifyEscrowClaimV065(e, claim, 11)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyEscrowRefundV065(claimed, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyEscrowReleaseV065(refunded, proof, 21); err == nil {
		t.Fatal("post-refund release accepted")
	}
}
