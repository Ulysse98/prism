package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func runReservedGrantPreview(
	args []string,
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) error {
	if len(args) != 3 {
		return fmt.Errorf(
			"usage: .\\prism.exe reserved grant-preview <pool> <recipient> <amount>",
		)
	}

	pool, err := parseReservedPool(args[0])
	if err != nil {
		return err
	}

	recipient, recipientName, err :=
		resolveAddress(
			args[1],
			wallets,
		)

	if err != nil {
		return err
	}

	var amount uint64

	_, err = fmt.Sscan(
		args[2],
		&amount,
	)

	if err != nil || amount == 0 {
		return fmt.Errorf(
			"reserved grant amount must be a positive integer",
		)
	}

	chainID, err := chain.ChainID()
	if err != nil {
		return err
	}

	nonce, err :=
		nextReservedGrantNonce(
			chain,
			pool,
		)

	if err != nil {
		return err
	}

	grant := reserved.NewGrant(
		chainID,
		nonce,
		pool,
		recipient,
		amount,
	)

	fmt.Println()
	fmt.Println("=== RESERVED GRANT PREVIEW ===")
	fmt.Println()

	fmt.Println("Pool:", pool)
	fmt.Println("Recipient:", recipientName)
	fmt.Printf("Amount: %d PRISM\n", amount)
	fmt.Printf("Nonce: %d\n", nonce)
	fmt.Println("Chain ID:", chainID)
	fmt.Println("Grant ID:", grant.ID)

	fmt.Println()
	fmt.Println(
		"Status: NOT EXECUTED",
	)

	return nil
}

func runReservedRevoke(
	args []string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	if len(args) < 4 {
		return fmt.Errorf(
			"usage: .\\prism.exe reserved revoke <pool> <grant-id> <authority> <authority> [...]",
		)
	}

	pool, err := parseReservedPool(args[0])
	if err != nil {
		return err
	}

	grantID := args[1]

	chainID, err := chain.ChainID()
	if err != nil {
		return err
	}

	policy :=
		chain.Config.ReservedAuthorities

	threshold, err :=
		policy.EffectiveThreshold(pool)

	if err != nil {
		return err
	}

	signers := args[2:]

	if uint32(len(signers)) < threshold {
		return fmt.Errorf(
			"not enough revocation signers: provided=%d threshold=%d",
			len(signers),
			threshold,
		)
	}

	revocation :=
		reserved.NewRevocation(
			chainID,
			pool,
			grantID,
		)

	for _, identifier := range signers {
		name, currentWallet, err :=
			resolveLocalWallet(
				identifier,
				wallets,
			)

		if err != nil {
			return err
		}

		authorized, err :=
			policy.IsAuthorized(
				pool,
				currentWallet.Address,
			)

		if err != nil {
			return err
		}

		if !authorized {
			return fmt.Errorf(
				"%s is not authorized for reserved pool %s",
				name,
				pool,
			)
		}

		if err := revocation.AddApproval(
			currentWallet.Address,
			currentWallet.PublicKeyHex(),
			currentWallet.PrivateKey,
		); err != nil {

			return fmt.Errorf(
				"revocation approval from %s rejected: %w",
				name,
				err,
			)
		}
	}

	if err := policy.ValidateRevocation(
		revocation,
		chainID,
	); err != nil {

		return fmt.Errorf(
			"reserved revocation authorization failed: %w",
			err,
		)
	}

	last :=
		chain.Blocks[len(chain.Blocks)-1]

	proposer, err :=
		pos.SelectProposer(
			last.Hash,
			last.Height+1,
		)

	if err != nil {
		return err
	}

	block, err :=
		chain.AddReservedRevocationBlock(
			[]reserved.Revocation{
				revocation,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		return fmt.Errorf(
			"reserved revocation block rejected: %w",
			err,
		)
	}

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {

		return err
	}

	fmt.Println()
	fmt.Println(
		"=== RESERVED GRANT REVOKED ===",
	)
	fmt.Println()

	fmt.Println("Pool:", pool)
	fmt.Println("Grant ID:", grantID)
	fmt.Println("Revocation ID:", revocation.ID)

	fmt.Printf(
		"Approvals: %d/%d\n",
		len(revocation.Approvals),
		threshold,
	)

	fmt.Printf(
		"Block: %d\n",
		block.Height,
	)

	fmt.Println(
		"PoS proposer:",
		walletNameForAddress(
			block.Proposer,
			wallets,
		),
	)

	fmt.Println(
		"Block hash:",
		block.Hash,
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	return nil
}

func runReservedRevocations(
	chain *blockchain.Blockchain,
) {
	fmt.Println(
		"=== PRISM RESERVED REVOCATIONS ===",
	)
	fmt.Println()

	found := false

	for _, block := range chain.Blocks {
		for _, revocation := range block.ReservedRevocations {

			found = true

			fmt.Printf(
				"Block:         %d\n",
				block.Height,
			)

			fmt.Printf(
				"Pool:          %s\n",
				revocation.Pool,
			)

			fmt.Printf(
				"Grant ID:      %s\n",
				revocation.GrantID,
			)

			fmt.Printf(
				"Revocation ID: %s\n",
				revocation.ID,
			)

			fmt.Printf(
				"Approvals:     %d\n",
				len(revocation.Approvals),
			)

			fmt.Println()
		}
	}

	if !found {
		fmt.Println(
			"No reserved revocations recorded yet.",
		)
	}
}
