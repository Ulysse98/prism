package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func parseReservedPool(
	value string,
) (consensus.ReservedPool, error) {
	switch strings.ToLower(value) {
	case "ecosystem":
		return consensus.ReservedPoolEcosystem, nil

	case "treasury":
		return consensus.ReservedPoolTreasury, nil

	case "team":
		return consensus.ReservedPoolTeam, nil

	case "liquidity":
		return consensus.ReservedPoolLiquidity, nil

	default:
		return "", fmt.Errorf(
			"unknown reserved pool: %s",
			value,
		)
	}
}

func nextReservedGrantNonce(
	chain *blockchain.Blockchain,
	pool consensus.ReservedPool,
) (uint64, error) {
	if chain == nil {
		return 0, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	var highest uint64

	for _, block := range chain.Blocks {
		for _, grant := range block.ReservedGrants {
			if grant.Pool != pool {
				continue
			}

			if grant.Nonce > highest {
				highest = grant.Nonce
			}
		}
	}

	if highest == math.MaxUint64 {
		return 0, fmt.Errorf(
			"reserved grant nonce exhausted for pool %s",
			pool,
		)
	}

	return highest + 1, nil
}

func runReservedGrantCreate(
	args []string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	if len(args) < 4 {
		return fmt.Errorf(
			"usage: .\\prism.exe reserved grant-create <pool> <recipient> <amount> <authority> [authority...]",
		)
	}

	if chain == nil {
		return fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if pos == nil {
		return fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if len(chain.Blocks) == 0 {
		return fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	pool, err := parseReservedPool(
		args[0],
	)
	if err != nil {
		return err
	}

	recipientAddress, recipientName, err :=
		resolveAddress(
			args[1],
			wallets,
		)

	if err != nil {
		return fmt.Errorf(
			"invalid grant recipient: %w",
			err,
		)
	}

	amount, err := strconv.ParseUint(
		args[2],
		10,
		64,
	)

	if err != nil || amount == 0 {
		return fmt.Errorf(
			"reserved grant amount must be a positive integer",
		)
	}

	policy :=
		chain.Config.ReservedAuthorities

	threshold, err :=
		policy.EffectiveThreshold(pool)

	if err != nil {
		return fmt.Errorf(
			"cannot determine reserved authority threshold: %w",
			err,
		)
	}

	signerIdentifiers := args[3:]

	if uint32(len(signerIdentifiers)) < threshold {
		return fmt.Errorf(
			"not enough grant signers: provided=%d threshold=%d",
			len(signerIdentifiers),
			threshold,
		)
	}

	chainID, err := chain.ChainID()
	if err != nil {
		return fmt.Errorf(
			"cannot determine chain ID: %w",
			err,
		)
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
		recipientAddress,
		amount,
	)

	for _, signerIdentifier := range signerIdentifiers {

		signerName, signerWallet, err :=
			resolveLocalWallet(
				signerIdentifier,
				wallets,
			)

		if err != nil {
			return fmt.Errorf(
				"invalid grant authority %s: %w",
				signerIdentifier,
				err,
			)
		}

		authorized, err :=
			policy.IsAuthorized(
				pool,
				signerWallet.Address,
			)

		if err != nil {
			return err
		}

		if !authorized {
			return fmt.Errorf(
				"%s is not authorized for reserved pool %s",
				signerName,
				pool,
			)
		}

		if err := grant.AddApproval(
			signerWallet.Address,
			signerWallet.PublicKeyHex(),
			signerWallet.PrivateKey,
		); err != nil {

			return fmt.Errorf(
				"approval from %s rejected: %w",
				signerName,
				err,
			)
		}
	}

	if err := policy.ValidateGrant(
		grant,
		chainID,
	); err != nil {

		return fmt.Errorf(
			"reserved grant authorization failed: %w",
			err,
		)
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	nextHeight :=
		lastBlock.Height + 1

	proposer, err :=
		pos.SelectProposer(
			lastBlock.Hash,
			nextHeight,
		)

	if err != nil {
		return fmt.Errorf(
			"cannot select grant block proposer: %w",
			err,
		)
	}

	block, err :=
		chain.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		return fmt.Errorf(
			"reserved grant block rejected: %w",
			err,
		)
	}

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {

		return fmt.Errorf(
			"unable to persist reserved grant block: %w",
			err,
		)
	}

	balance, err :=
		chain.BalanceOf(
			recipientAddress,
		)

	if err != nil {
		return fmt.Errorf(
			"unable to read recipient balance: %w",
			err,
		)
	}

	emission, err :=
		chain.ReservedEmission()

	if err != nil {
		return fmt.Errorf(
			"unable to read reserved emission: %w",
			err,
		)
	}

	fmt.Println()
	fmt.Println(
		"=== RESERVED GRANT CONFIRMED ===",
	)
	fmt.Println()

	fmt.Println(
		"Pool:",
		pool,
	)

	fmt.Println(
		"Recipient:",
		recipientName,
	)

	fmt.Println(
		"Address:",
		shortAddress(recipientAddress),
	)

	fmt.Printf(
		"Amount: %d PRISM\n",
		grant.Amount,
	)

	fmt.Printf(
		"Nonce: %d\n",
		grant.Nonce,
	)

	fmt.Printf(
		"Approvals: %d/%d\n",
		len(grant.Approvals),
		threshold,
	)

	fmt.Println(
		"Grant ID:",
		grant.ID,
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
		"Recipient balance: %d PRISM\n",
		balance,
	)

	fmt.Printf(
		"Reserved emission: %d PRISM\n",
		emission,
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	return nil
}
