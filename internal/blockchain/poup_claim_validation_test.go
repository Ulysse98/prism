package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/poup"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func buildPoUPClaimValidationChain(
	t *testing.T,
	blockCount int,
) (
	*Blockchain,
	*consensus.ProofOfStake,
	*wallet.Wallet,
) {
	t.Helper()

	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := NewBlockchain(
		map[string]uint64{
			actor.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 100

	if err := chain.LockStake(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	attestation, err :=
		identity.NewWorldIDAttestation(
			actor.Address,
			"poup-claim-test-nullifier",
			"prism-poup-claim-test",
		)

	if err != nil {
		t.Fatal(err)
	}

	for index := 1; index <= blockCount; index++ {
		task, err :=
			usefulwork.NewSumSquaresTask(
				[]uint64{
					uint64(index),
				},
			)

		if err != nil {
			t.Fatal(err)
		}

		proof, err :=
			usefulwork.Execute(
				task,
				actor,
			)

		if err != nil {
			t.Fatal(err)
		}

		if index == 1 {
			if _, err := chain.addBlock(
				nil,
				[]usefulwork.Proof{
					proof,
				},
				[]identity.Attestation{
					attestation,
				},
				actor.Address,
				pos,
			); err != nil {
				t.Fatal(err)
			}

			continue
		}

		if _, err := chain.AddBlock(
			nil,
			[]usefulwork.Proof{
				proof,
			},
			actor.Address,
			pos,
		); err != nil {
			t.Fatalf(
				"failed to add block %d: %v",
				index,
				err,
			)
		}
	}

	return chain, pos, actor
}

func signedPoUPClaim(
	t *testing.T,
	actor *wallet.Wallet,
	points uint64,
	units uint64,
	amount uint64,
) poup.Claim {
	t.Helper()

	claim := poup.NewClaim(
		actor.Address,
		0,
		points,
		units,
		amount,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		actor.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	return claim
}

func appendPoUPClaimBlock(
	t *testing.T,
	chain *Blockchain,
	actor *wallet.Wallet,
	claims []poup.Claim,
) {
	t.Helper()

	previous :=
		chain.Blocks[len(chain.Blocks)-1]

	block := Block{
		Height: previous.Height + 1,
		Timestamp: previous.Timestamp.Add(
			time.Second,
		),
		PreviousHash: previous.Hash,
		Proposer:     actor.Address,
		Reward: consensus.DefaultRewardPolicy().
			ProposerReward,
		ParticipationClaims: claims,
	}

	block.Hash =
		CalculateHash(block)

	chain.Blocks = append(
		chain.Blocks,
		block,
	)
}

func TestValidPoUPClaimPassesConsensusValidation(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	claim := signedPoUPClaim(
		t,
		actor,
		1200,
		10,
		10,
	)

	if err := chain.validateParticipationClaim(
		claim,
		101,
		consensus.DefaultRewardPolicy(),
	); err != nil {
		t.Fatal(err)
	}

	before, err := chain.BalanceOf(
		actor.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	appendPoUPClaimBlock(
		t,
		chain,
		actor,
		[]poup.Claim{
			claim,
		},
	)

	if !chain.ValidateChain(pos) {
		t.Fatal(
			"expected chain with valid PoUP claim to validate",
		)
	}

	after, err := chain.BalanceOf(
		actor.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	expected :=
		before +
			consensus.DefaultRewardPolicy().
				ProposerReward

	if after != expected {
		t.Fatalf(
			"PoUP claim credited balance too early: expected %d, got %d",
			expected,
			after,
		)
	}
}

func TestPoUPClaimRejectsIncompletePeriod(
	t *testing.T,
) {
	chain, _, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	claim := poup.NewClaim(
		actor.Address,
		1,
		10,
		1,
		1,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		actor.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err := chain.validateParticipationClaim(
		claim,
		101,
		consensus.DefaultRewardPolicy(),
	); err == nil {
		t.Fatal(
			"expected incomplete reward period to fail",
		)
	}
}

func TestPoUPClaimRejectsIncorrectEconomics(
	t *testing.T,
) {
	chain, _, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	tests := []struct {
		name   string
		points uint64
		units  uint64
		amount uint64
	}{
		{
			name:   "points",
			points: 1190,
			units:  10,
			amount: 10,
		},
		{
			name:   "units",
			points: 1200,
			units:  9,
			amount: 9,
		},
		{
			name:   "amount",
			points: 1200,
			units:  10,
			amount: 9,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				claim := signedPoUPClaim(
					t,
					actor,
					test.points,
					test.units,
					test.amount,
				)

				if err :=
					chain.validateParticipationClaim(
						claim,
						101,
						consensus.DefaultRewardPolicy(),
					); err == nil {

					t.Fatal(
						"expected incorrect PoUP economics to fail",
					)
				}
			},
		)
	}
}

func TestPoUPClaimRejectsDuplicateAddressPeriod(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	claim := signedPoUPClaim(
		t,
		actor,
		1200,
		10,
		10,
	)

	appendPoUPClaimBlock(
		t,
		chain,
		actor,
		[]poup.Claim{
			claim,
			claim,
		},
	)

	if chain.ValidateChain(pos) {
		t.Fatal(
			"expected duplicate address-period claim to invalidate chain",
		)
	}
}
