package compute

import (
	"strings"
	"testing"

	"prism/internal/wallet"
)

func TestClaimAuthorizationRoundTrip(
	t *testing.T,
) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim, err := SignClaimAuthorization(
		strings.Repeat("a", 64),
		"prism-test-chain",
		strings.Repeat("b", 64),
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyClaimAuthorization(
		claim,
		claim.JobID,
		claim.ChainID,
		claim.GenesisHash,
	); err != nil {
		t.Fatalf(
			"valid claim rejected: %v",
			err,
		)
	}
}

func TestClaimAuthorizationRejectsContextReplay(
	t *testing.T,
) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	jobID := strings.Repeat("a", 64)
	chainID := "prism-test-chain"
	genesisHash := strings.Repeat("b", 64)

	claim, err := SignClaimAuthorization(
		jobID,
		chainID,
		genesisHash,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		jobID       string
		chainID     string
		genesisHash string
	}{
		{
			name:        "other job",
			jobID:       strings.Repeat("c", 64),
			chainID:     chainID,
			genesisHash: genesisHash,
		},
		{
			name:        "other chain",
			jobID:       jobID,
			chainID:     "prism-other-chain",
			genesisHash: genesisHash,
		},
		{
			name:        "other genesis",
			jobID:       jobID,
			chainID:     chainID,
			genesisHash: strings.Repeat("d", 64),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				if err := VerifyClaimAuthorization(
					claim,
					test.jobID,
					test.chainID,
					test.genesisHash,
				); err == nil {
					t.Fatal(
						"expected replay context rejection",
					)
				}
			},
		)
	}
}

func TestClaimAuthorizationRejectsWorkerImpersonation(
	t *testing.T,
) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	victim, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim, err := SignClaimAuthorization(
		strings.Repeat("a", 64),
		"prism-test-chain",
		strings.Repeat("b", 64),
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	claim.Worker = victim.Address
	claim.ID = CalculateClaimAuthorizationID(
		claim,
	)

	if err := VerifyClaimAuthorization(
		claim,
		claim.JobID,
		claim.ChainID,
		claim.GenesisHash,
	); err == nil {
		t.Fatal(
			"expected worker impersonation rejection",
		)
	}
}

func TestClaimAuthorizationRejectsBadSignature(
	t *testing.T,
) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim, err := SignClaimAuthorization(
		strings.Repeat("a", 64),
		"prism-test-chain",
		strings.Repeat("b", 64),
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	claim.Signature =
		"00" + claim.Signature[2:]

	if err := VerifyClaimAuthorization(
		claim,
		claim.JobID,
		claim.ChainID,
		claim.GenesisHash,
	); err == nil {
		t.Fatal(
			"expected invalid signature rejection",
		)
	}
}
