package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func buildReservedBlockTestChain(
	t *testing.T,
) (
	*Blockchain,
	*consensus.ProofOfStake,
	*wallet.Wallet,
) {
	t.Helper()

	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := chain.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	return chain, pos, validator
}

func TestAddReservedAuthorizationBlock(
	t *testing.T,
) {
	chain, pos, validator :=
		buildReservedBlockTestChain(t)

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorizer.Address,
			},
		},
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			chain,
			authorizer,
		)

	before, err := chain.ReservedEmission()
	if err != nil {
		t.Fatal(err)
	}

	block, err :=
		chain.AddReservedAuthorizationBlock(
			[]reserved.Authorization{
				authorization,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height != 1 {
		t.Fatalf(
			"expected reserved block height 1, got %d",
			block.Height,
		)
	}

	if len(block.ReservedAuthorizations) != 1 {
		t.Fatalf(
			"expected one reserved authorization, got %d",
			len(block.ReservedAuthorizations),
		)
	}

	if block.ReservedAuthorizations[0].ID !=
		authorization.ID {

		t.Fatal(
			"produced block did not preserve authorization",
		)
	}

	if !chain.ValidateChain(pos) {
		t.Fatal(
			"expected produced reserved block to validate",
		)
	}

	balance, err :=
		chain.BalanceOf(
			authorization.Recipient,
		)

	if err != nil {
		t.Fatal(err)
	}

	if balance != authorization.Amount {
		t.Fatalf(
			"expected reserved recipient balance %d, got %d",
			authorization.Amount,
			balance,
		)
	}

	after, err := chain.ReservedEmission()
	if err != nil {
		t.Fatal(err)
	}

	expected :=
		before + authorization.Amount

	if after != expected {
		t.Fatalf(
			"expected reserved emission %d, got %d",
			expected,
			after,
		)
	}
}

func TestAddReservedAuthorizationBlockRejectsEmpty(
	t *testing.T,
) {
	chain, pos, validator :=
		buildReservedBlockTestChain(t)

	before := len(chain.Blocks)

	if _, err :=
		chain.AddReservedAuthorizationBlock(
			nil,
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected empty reserved block to fail",
		)
	}

	if len(chain.Blocks) != before {
		t.Fatal(
			"failed reserved block mutated chain",
		)
	}
}

func TestAddReservedAuthorizationBlockRejectsInvalidAuthorization(
	t *testing.T,
) {
	chain, pos, validator :=
		buildReservedBlockTestChain(t)

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorizer.Address,
			},
		},
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			chain,
			authorizer,
		)

	authorization.Amount++

	before := len(chain.Blocks)

	if _, err :=
		chain.AddReservedAuthorizationBlock(
			[]reserved.Authorization{
				authorization,
			},
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected invalid reserved authorization block to fail",
		)
	}

	if len(chain.Blocks) != before {
		t.Fatal(
			"invalid reserved block mutated chain",
		)
	}
}
