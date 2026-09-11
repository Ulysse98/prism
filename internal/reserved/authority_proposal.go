package reserved

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"prism/internal/consensus"
)

// AuthorityProposal represents a queued governance proposal.
//
// The proposal deliberately does not contain ProposalHeight or
// ExecuteAfterHeight. Those values are consensus-derived when the
// proposal is included on-chain.
//
// The embedded AuthorityChange contains the actual governance intent
// and its authority approvals.
type AuthorityProposal struct {
	ID string `json:"id"`

	Change AuthorityChange `json:"change"`
}

// NewAuthorityProposal creates a queued governance proposal.
//
// Queued governance uses a legacy-style AuthorityChange with
// ActivationHeight == 0. The execution delay is enforced by the
// proposal queue rather than by the v0.28 direct timelock mechanism.
func NewAuthorityProposal(
	chainID string,
	nonce uint64,
	pool consensus.ReservedPool,
	action AuthorityChangeAction,
	authority string,
) AuthorityProposal {
	proposal := AuthorityProposal{
		Change: NewAuthorityChange(
			chainID,
			nonce,
			pool,
			action,
			authority,
		),
	}

	proposal.ID =
		CalculateAuthorityProposalID(
			proposal,
		)

	return proposal
}

func authorityProposalPayload(
	proposal AuthorityProposal,
) string {
	return fmt.Sprintf(
		"reserved-authority-proposal-v1|%s",
		proposal.Change.ID,
	)
}

func CalculateAuthorityProposalID(
	proposal AuthorityProposal,
) string {
	hash := sha256.Sum256(
		[]byte(
			authorityProposalPayload(
				proposal,
			),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func ValidateAuthorityProposal(
	proposal AuthorityProposal,
) error {
	if err :=
		ValidateAuthorityChange(
			proposal.Change,
		); err != nil {

		return fmt.Errorf(
			"invalid reserved authority proposal change: %w",
			err,
		)
	}

	// v0.29 queued governance owns the timing lifecycle.
	// A proposal must therefore not also carry the v0.28
	// direct ActivationHeight mechanism.
	if proposal.Change.ActivationHeight != 0 {
		return fmt.Errorf(
			"queued authority proposal cannot contain direct activation height",
		)
	}

	if proposal.ID == "" {
		return fmt.Errorf(
			"reserved authority proposal ID cannot be empty",
		)
	}

	if proposal.ID !=
		CalculateAuthorityProposalID(
			proposal,
		) {

		return fmt.Errorf(
			"invalid reserved authority proposal ID",
		)
	}

	return nil
}

// AddApproval adds an authority approval to the proposal's underlying
// governance change.
//
// AuthorityChange IDs do not depend on approvals, so adding approvals
// does not mutate the proposal ID.
func (proposal *AuthorityProposal) AddApproval(
	authorizer string,
	publicKey string,
	privateKey ed25519.PrivateKey,
) error {
	if proposal == nil {
		return fmt.Errorf(
			"reserved authority proposal cannot be nil",
		)
	}

	if err :=
		ValidateAuthorityProposal(
			*proposal,
		); err != nil {

		return err
	}

	return proposal.Change.AddApproval(
		authorizer,
		publicKey,
		privateKey,
	)
}

// ValidateAuthorityProposalApproval validates one approval attached to
// the proposal's underlying authority change.
func ValidateAuthorityProposalApproval(
	proposal AuthorityProposal,
	approval Approval,
) error {
	if err :=
		ValidateAuthorityProposal(
			proposal,
		); err != nil {

		return err
	}

	return ValidateAuthorityChangeApproval(
		proposal.Change,
		approval,
	)
}
