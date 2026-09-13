package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/participation"
	"prism/internal/wallet"
)

func runParticipate(
	participantIdentifier string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	address, participantName, err := resolveAddress(
		participantIdentifier,
		wallets,
	)
	if err != nil {
		return err
	}

	// PoUP is restricted to humanity-verified Prism addresses.
	if !chain.IsVerified(address) {
		return fmt.Errorf(
			"human verification required for %s",
			participantName,
		)
	}

	// The blockchain itself implements IsVerified(address),
	// therefore it acts as the PoUP eligibility checker.
	scores, err := participation.Calculate(
		chain,
		pos,
		chain,
	)
	if err != nil {
		return err
	}

	score := participation.ScoreOf(
		scores,
		address,
	)

	if score == 0 {
		return fmt.Errorf(
			"no useful participation recorded yet for %s",
			participantName,
		)
	}

	fmt.Println("=== PROOF OF USEFUL PARTICIPATION ===")
	fmt.Println()
	fmt.Println("Participant:", participantName)
	fmt.Println("Address:", shortAddress(address))
	fmt.Println("Humanity: VERIFIED")
	fmt.Printf("Participation score: %d\n", score)
	fmt.Println("Eligibility: ACCEPTED")
	fmt.Println()
	fmt.Println("Eligible for PoUP reward: YES")

	return nil
}
