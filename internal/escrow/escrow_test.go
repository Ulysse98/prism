package escrow

import (
	"strings"
	"testing"
)

func sampleFunding() Funding {
	f := Funding{
		ChainID:         "prism-" + strings.Repeat("a", 16),
		GenesisHash:     strings.Repeat("b", 64),
		TaskID:          strings.Repeat("c", 64),
		JobNonce:        9,
		FundingTxID:     strings.Repeat("d", 64),
		Requester:       "prism_" + strings.Repeat("1", 40),
		Amount:          10,
		FundedAtHeight:  10,
		ExpiresAtHeight: 20,
	}
	f.JobID = CalculateFundedJobID(f.TaskID, f.Requester, f.Amount, f.JobNonce)
	return f
}

func sampleProof(f Funding) ProofReference {
	return ProofReference{
		ID: strings.Repeat("e", 64), ChainID: f.ChainID,
		GenesisHash: f.GenesisHash, JobID: f.JobID,
		Worker: "prism_" + strings.Repeat("2", 40),
	}
}

func TestFundClaimReleaseExactlyOnce(t *testing.T) {
	f := sampleFunding()
	e, err := NewFunded(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Claim(sampleProof(f).Worker, 11); err != nil {
		t.Fatal(err)
	}
	if err := e.Release(sampleProof(f), 19); err != nil {
		t.Fatal(err)
	}
	if e.Status != Released || e.ProofID == "" {
		t.Fatalf("unexpected release: %+v", e)
	}
	if err := e.Release(sampleProof(f), 19); err == nil {
		t.Fatal("duplicate release accepted")
	}
	if err := e.Refund(20); err == nil {
		t.Fatal("refund after release accepted")
	}
}

func TestExpiryRefundAndNoLateRelease(t *testing.T) {
	f := sampleFunding()
	e, err := NewFunded(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Claim(sampleProof(f).Worker, 12); err != nil {
		t.Fatal(err)
	}
	if err := e.Refund(19); err == nil {
		t.Fatal("early refund accepted")
	}
	if err := e.Release(sampleProof(f), 20); err == nil {
		t.Fatal("late release accepted")
	}
	if e.Status != Claimed {
		t.Fatal("failed transition mutated escrow")
	}
	if err := e.Refund(20); err != nil {
		t.Fatal(err)
	}
	if e.Status != Refunded {
		t.Fatalf("unexpected status: %s", e.Status)
	}
	if err := e.Claim(sampleProof(f).Worker, 21); err == nil {
		t.Fatal("claim after refund accepted")
	}
	if err := e.Refund(21); err == nil {
		t.Fatal("duplicate refund accepted")
	}
}

func TestClaimGuardsAndProofBinding(t *testing.T) {
	f := sampleFunding()
	e, err := NewFunded(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Claim(f.Requester, 11); err == nil {
		t.Fatal("self claim accepted")
	}
	if err := e.Claim(sampleProof(f).Worker, 20); err == nil {
		t.Fatal("expired claim accepted")
	}
	if err := e.Claim(sampleProof(f).Worker, 11); err != nil {
		t.Fatal(err)
	}
	if err := e.Claim(sampleProof(f).Worker, 12); err == nil {
		t.Fatal("double claim accepted")
	}
	wrong := sampleProof(f)
	wrong.JobID = strings.Repeat("f", 64)
	if err := e.Release(wrong, 12); err == nil {
		t.Fatal("foreign job proof accepted")
	}
	wrong = sampleProof(f)
	wrong.Worker = f.Requester
	if err := e.Release(wrong, 12); err == nil {
		t.Fatal("foreign worker proof accepted")
	}
	wrong = sampleProof(f)
	wrong.ChainID = "prism-" + strings.Repeat("b", 16)
	if err := e.Release(wrong, 12); err == nil {
		t.Fatal("foreign chain proof accepted")
	}
	if e.Status != Claimed || e.ProofID != "" {
		t.Fatal("failed release mutated escrow")
	}
}

func TestFundingValidationAndContextBoundID(t *testing.T) {
	base := sampleFunding()
	e, err := NewFunded(base)
	if err != nil {
		t.Fatal(err)
	}
	again, err := NewFunded(base)
	if err != nil || again.ID != e.ID {
		t.Fatal("non-deterministic escrow ID")
	}
	changed := base
	changed.FundingTxID = strings.Repeat("e", 64)
	different, err := NewFunded(changed)
	if err != nil || different.ID == e.ID {
		t.Fatal("escrow ID not bound to funding tx")
	}
	invalid := []Funding{
		func() Funding { x := base; x.Amount = 0; return x }(),
		func() Funding { x := base; x.ExpiresAtHeight = 10; return x }(),
		func() Funding { x := base; x.ChainID = "other"; return x }(),
		func() Funding { x := base; x.Requester = "Alice"; return x }(),
		func() Funding { x := base; x.JobID = "incorrect"; return x }(),
		func() Funding { x := base; x.GenesisHash = "bad"; return x }(),
	}
	for i, spec := range invalid {
		if _, err := NewFunded(spec); err == nil {
			t.Fatalf("invalid funding %d accepted", i)
		}
	}
	e.ID = strings.Repeat("f", 64)
	if err := e.Claim(sampleProof(base).Worker, 12); err == nil {
		t.Fatal("tampered escrow ID accepted")
	}
}
