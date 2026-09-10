package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func runReservedCommand(
	args []string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) {
	if len(args) == 0 {
		printReservedUsage()
		return
	}

	switch args[0] {
	case "pools":
		runReservedPools()

	case "authorities":
		runReservedAuthorities(chain, wallets)

	case "grants":
		runReservedGrants(chain, wallets)

	case "grant-preview":
		if err := runReservedGrantPreview(
			args[1:],
			chain,
			wallets,
		); err != nil {
			fmt.Println("Reserved grant preview rejected:")
			fmt.Println(err)
		}

	case "revocations":
		runReservedRevocations(chain)

	case "revoke":
		if err := runReservedRevoke(
			args[1:],
			chain,
			pos,
			wallets,
		); err != nil {
			fmt.Println("Reserved revocation rejected:")
			fmt.Println(err)
		}

	case "grant-create":
		if err := runReservedGrantCreate(
			args[1:],
			chain,
			pos,
			wallets,
		); err != nil {
			fmt.Println(
				"Reserved grant rejected:",
			)
			fmt.Println(err)
		}

	case "help":
		printReservedUsage()

	default:
		fmt.Printf(
			"Unknown reserved command: %s\n\n",
			args[0],
		)

		printReservedUsage()
	}
}

func configureDevReservedAuthorities(
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) error {
	if chain == nil {
		return fmt.Errorf("blockchain cannot be nil")
	}

	names := []string{
		"Alice",
		"Bob",
		"Charlie",
	}

	authorities := make(
		[]string,
		0,
		len(names),
	)

	for _, name := range names {
		currentWallet := wallets[name]

		if currentWallet == nil {
			return fmt.Errorf(
				"reserved authority wallet missing: %s",
				name,
			)
		}

		authorities = append(
			authorities,
			currentWallet.Address,
		)
	}

	copyAuthorities := func() []string {
		return append(
			[]string(nil),
			authorities...,
		)
	}

	config := blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Ecosystem: copyAuthorities(),
			Treasury:  copyAuthorities(),
			Team:      copyAuthorities(),
			Liquidity: copyAuthorities(),

			EcosystemThreshold: 2,
			TreasuryThreshold:  2,
			TeamThreshold:      2,
			LiquidityThreshold: 2,
		},
	}

	canonical, err := config.Canonical()
	if err != nil {
		return err
	}

	chain.Config = canonical

	return nil
}

func runReservedPools() {
	policy := consensus.DefaultSupplyPolicy()

	pools := []consensus.ReservedPool{
		consensus.ReservedPoolEcosystem,
		consensus.ReservedPoolTreasury,
		consensus.ReservedPoolTeam,
		consensus.ReservedPoolLiquidity,
	}

	fmt.Println("=== PRISM RESERVED POOLS ===")
	fmt.Println()

	var total uint64

	for _, pool := range pools {
		allocation, err :=
			policy.ReservedPoolAllocation(pool)

		if err != nil {
			fmt.Printf(
				"%s: error: %v\n",
				pool,
				err,
			)

			continue
		}

		total += allocation

		fmt.Printf(
			"%-10s %12d PRISM\n",
			pool,
			allocation,
		)
	}

	fmt.Println()

	fmt.Printf(
		"Reserved total: %d PRISM\n",
		total,
	)

	fmt.Printf(
		"Max supply:     %d PRISM\n",
		policy.MaxSupply,
	)
}

func runReservedAuthorities(
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) {
	fmt.Println("=== PRISM RESERVED AUTHORITIES ===")
	fmt.Println()

	if chain == nil {
		fmt.Println("No blockchain state available.")
		return
	}

	chainID, err := chain.ChainID()
	if err != nil {
		fmt.Println("Chain ID error:", err)
		return
	}

	fmt.Println("Chain ID:", chainID)
	fmt.Println()

	policy := chain.Config.ReservedAuthorities

	type poolAuthorities struct {
		pool        consensus.ReservedPool
		authorities []string
	}

	pools := []poolAuthorities{
		{
			pool:        consensus.ReservedPoolEcosystem,
			authorities: policy.Ecosystem,
		},
		{
			pool:        consensus.ReservedPoolTreasury,
			authorities: policy.Treasury,
		},
		{
			pool:        consensus.ReservedPoolTeam,
			authorities: policy.Team,
		},
		{
			pool:        consensus.ReservedPoolLiquidity,
			authorities: policy.Liquidity,
		},
	}

	for _, entry := range pools {
		fmt.Printf(
			"Pool: %s\n",
			entry.pool,
		)

		threshold, err :=
			policy.EffectiveThreshold(
				entry.pool,
			)

		if err != nil {
			fmt.Println(
				"  Authorities: not configured",
			)
			fmt.Println()
			continue
		}

		fmt.Printf(
			"  Threshold: %d/%d\n",
			threshold,
			len(entry.authorities),
		)

		for _, address := range entry.authorities {
			fmt.Printf(
				"  - %-8s %s\n",
				walletNameForAddress(
					address,
					wallets,
				),
				shortAddress(address),
			)
		}

		fmt.Println()
	}
}

func runReservedGrants(
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) {
	fmt.Println("=== PRISM RESERVED GRANTS ===")
	fmt.Println()

	if chain == nil ||
		len(chain.Blocks) == 0 {

		fmt.Println(
			"No blockchain state available.",
		)

		return
	}

	currentHeight :=
		chain.Blocks[len(chain.Blocks)-1].Height

	revoked := make(
		map[string]bool,
	)

	for _, block := range chain.Blocks {
		for _, revocation := range block.ReservedRevocations {

			revoked[revocation.GrantID] = true
		}
	}

	found := false

	for _, block := range chain.Blocks {
		for _, grant := range block.ReservedGrants {

			found = true

			status := reservedGrantStatus(
				grant.NotBeforeHeight,
				grant.ExpiresAtHeight,
				currentHeight,
				revoked[grant.ID],
			)

			fmt.Printf(
				"Block:       %d\n",
				block.Height,
			)

			fmt.Printf(
				"Grant ID:    %s\n",
				grant.ID,
			)

			fmt.Printf(
				"Pool:        %s\n",
				grant.Pool,
			)

			fmt.Printf(
				"Recipient:   %s\n",
				walletNameForAddress(
					grant.Recipient,
					wallets,
				),
			)

			fmt.Printf(
				"Address:     %s\n",
				shortAddress(
					grant.Recipient,
				),
			)

			fmt.Printf(
				"Amount:      %d PRISM\n",
				grant.Amount,
			)

			fmt.Printf(
				"Nonce:       %d\n",
				grant.Nonce,
			)

			fmt.Printf(
				"Approvals:   %d\n",
				len(grant.Approvals),
			)

			fmt.Printf(
				"Chain ID:    %s\n",
				grant.ChainID,
			)

			if grant.NotBeforeHeight != 0 {
				fmt.Printf(
					"Not before:  %d\n",
					grant.NotBeforeHeight,
				)
			}

			if grant.ExpiresAtHeight != 0 {
				fmt.Printf(
					"Expires:     %d\n",
					grant.ExpiresAtHeight,
				)
			}

			fmt.Printf(
				"Status:      %s\n",
				status,
			)

			fmt.Println()
		}
	}

	if !found {
		fmt.Println(
			"No reserved grants recorded yet.",
		)
	}
}

func reservedGrantStatus(
	notBefore uint64,
	expires uint64,
	currentHeight uint64,
	revoked bool,
) string {
	if revoked {
		return "REVOKED"
	}

	if notBefore != 0 &&
		currentHeight < notBefore {

		return "PENDING"
	}

	if expires != 0 &&
		currentHeight > expires {

		return "EXPIRED"
	}

	return "ACTIVE"
}

func printReservedUsage() {
	fmt.Println("Reserved commands:")
	fmt.Println()

	fmt.Println(
		`  .\prism.exe reserved pools`,
	)

	fmt.Println(
		`  .\prism.exe reserved authorities`,
	)

	fmt.Println(
		`  .\prism.exe reserved grants`,
	)

	fmt.Println(
		`  .\prism.exe reserved grant-create treasury Bob 100 Alice Charlie`,
	)

	fmt.Println(
		`  .\prism.exe reserved grant-preview treasury Bob 50`,
	)

	fmt.Println(
		`  .\prism.exe reserved revoke treasury <grant-id> Alice Bob`,
	)

	fmt.Println(
		`  .\prism.exe reserved revocations`,
	)

	fmt.Println(
		`  .\prism.exe reserved help`,
	)
}
