package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestV061LegacyEmptyTeamRejectsValidatorAuthority(t *testing.T) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := NewBlockchain(map[string]uint64{
		validator.Address: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}

	chainID, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	proposal := reserved.NewAuthorityProposal(
		chainID,
		1,
		consensus.ReservedPoolTeam,
		reserved.AuthorityChangeAdd,
		recipient.Address,
	)

	if err := proposal.AddApproval(
		validator.Address,
		validator.PublicKeyHex(),
		validator.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	governance, err := chain.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}

	err = governance.QueueAuthorityProposal(
		proposal,
		chainID,
		3,
		reserved.DefaultGovernanceDelayBlocks,
	)

	if err == nil {
		t.Fatal("unauthorized validator approval was accepted")
	}

	if _, exists := governance.GetPendingAuthorityProposal(
		proposal.ID,
	); exists {
		t.Fatal("rejected proposal was queued")
	}
}
