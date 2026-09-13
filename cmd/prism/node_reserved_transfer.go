package main

import (
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func runNodeReservedTransferProposeCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node-transfer-propose",
		flag.ContinueOnError,
	)

	port := flags.Int(
		"port",
		7001,
		"local node data port",
	)

	peer := flags.String(
		"peer",
		"",
		"destination Prism peer for block broadcast",
	)

	nodeData := flags.String(
		"data",
		"",
		"node data directory",
	)

	chainConfig := flags.String(
		"chain-config",
		"",
		"JSON chain config used to create or verify node state",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid port:",
			*port,
		)
		return
	}

	positional := flags.Args()

	if len(positional) != 4 {
		fmt.Println("Usage:")
		fmt.Println(
			`prism node-transfer-propose --port 7001 --peer 127.0.0.1:7002 treasury Bob 100 1`,
		)
		fmt.Println()
		fmt.Println(
			"Arguments: <pool> <recipient> <amount> <nonce>",
		)
		return
	}

	pool, err :=
		parseGovernancePool(
			positional[0],
		)

	if err != nil {
		fmt.Println(
			"Invalid reserved transfer pool:",
			err,
		)
		return
	}

	amount, err :=
		strconv.ParseUint(
			positional[2],
			10,
			64,
		)

	if err != nil || amount == 0 {
		fmt.Println(
			"Reserved transfer amount must be greater than zero.",
		)
		return
	}

	nonce, err :=
		strconv.ParseUint(
			positional[3],
			10,
			64,
		)

	if err != nil || nonce == 0 {
		fmt.Println(
			"Reserved transfer nonce must be greater than zero.",
		)
		return
	}

	dataPath :=
		resolveNodeDataPath(
			*port,
			*nodeData,
		)

	chain, pos, wallets, created, err :=
		loadOrCreateP2PState(
			dataPath,
			*chainConfig,
		)

	if err != nil {
		fmt.Println(
			"Unable to load node state:",
		)
		fmt.Println(err)
		return
	}

	if created {
		fmt.Println(
			"Created new node state:",
			dataPath,
		)
	} else {
		fmt.Println(
			"Loaded node state:",
			dataPath,
		)
	}

	if !chain.ValidateChain(pos) {
		fmt.Println(
			"Refusing reserved transfer proposal from invalid local chain.",
		)
		return
	}

	recipientAddress, recipientLabel, err :=
		resolveAddress(
			positional[1],
			wallets,
		)

	if err != nil {
		fmt.Println(
			"Invalid reserved transfer recipient:",
		)
		fmt.Println(err)
		return
	}

	chainID, err :=
		chain.ChainID()

	if err != nil {
		fmt.Println(
			"Unable to determine Chain ID:",
			err,
		)
		return
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			chainID,
			nonce,
			pool,
			recipientAddress,
			amount,
		)

	governanceState, err :=
		chain.GetGovernanceState()

	if err != nil {
		fmt.Println(
			"Unable to reconstruct governance state:",
			err,
		)
		return
	}

	threshold, err :=
		governanceState.CurrentPolicy.EffectiveThreshold(
			pool,
		)

	if err != nil {
		fmt.Println(
			"Unable to determine reserved transfer threshold:",
			err,
		)
		return
	}

	approvedBy, err :=
		addLocalReservedTransferApprovals(
			&proposal,
			governanceState.CurrentPolicy,
			pool,
			threshold,
			wallets,
		)

	if err != nil {
		fmt.Println(
			"Unable to authorize reserved transfer proposal:",
		)
		fmt.Println(err)
		return
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
		fmt.Println(
			"Unable to select proposer:",
			err,
		)
		return
	}

	block, err :=
		chain.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		fmt.Println(
			"Reserved transfer proposal block rejected:",
		)
		fmt.Println(err)
		return
	}

	if err :=
		storage.Save(
			dataPath,
			chain,
			pos,
			wallets,
		); err != nil {

		fmt.Println(
			"Unable to save reserved transfer proposal block:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== GOVERNED RESERVED TRANSFER PROPOSAL CONFIRMED ===",
	)

	fmt.Println(
		"Proposal ID:",
		proposal.ID,
	)

	fmt.Println(
		"Pool:",
		pool,
	)

	fmt.Println(
		"Recipient:",
		recipientLabel,
	)

	fmt.Println(
		"Recipient address:",
		recipientAddress,
	)

	fmt.Println(
		"Amount:",
		amount,
		"PRISM",
	)

	fmt.Println(
		"Nonce:",
		nonce,
	)

	fmt.Printf(
		"Approvals: %d/%d\n",
		len(approvedBy),
		threshold,
	)

	for _, name := range approvedBy {
		fmt.Println(
			"  -",
			name,
		)
	}

	fmt.Println(
		"Proposal height:",
		block.Height,
	)

	fmt.Println(
		"Executable from height:",
		block.Height+
			reserved.DefaultGovernanceDelayBlocks,
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

	if *peer != "" {
		if err :=
			broadcastGovernanceBlock(
				*port,
				*peer,
				dataPath,
				chain,
				pos,
				wallets,
				block,
			); err != nil {

			fmt.Println(
				"Reserved transfer block broadcast failed:",
			)
			fmt.Println(err)
			return
		}

		fmt.Println(
			"Block submission: ACCEPTED",
		)
	}
}

func runNodeReservedTransferExecuteCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node-transfer-execute",
		flag.ContinueOnError,
	)

	port := flags.Int(
		"port",
		7001,
		"local node data port",
	)

	peer := flags.String(
		"peer",
		"",
		"destination Prism peer for block broadcast",
	)

	nodeData := flags.String(
		"data",
		"",
		"node data directory",
	)

	chainConfig := flags.String(
		"chain-config",
		"",
		"JSON chain config used to create or verify node state",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid port:",
			*port,
		)
		return
	}

	positional := flags.Args()

	if len(positional) != 1 {
		fmt.Println("Usage:")
		fmt.Println(
			`prism node-transfer-execute --port 7001 --peer 127.0.0.1:7002 <proposal-id>`,
		)
		return
	}

	proposalID :=
		strings.TrimSpace(
			positional[0],
		)

	if proposalID == "" {
		fmt.Println(
			"Proposal ID cannot be empty.",
		)
		return
	}

	dataPath :=
		resolveNodeDataPath(
			*port,
			*nodeData,
		)

	chain, pos, wallets, created, err :=
		loadOrCreateP2PState(
			dataPath,
			*chainConfig,
		)

	if err != nil {
		fmt.Println(
			"Unable to load node state:",
		)
		fmt.Println(err)
		return
	}

	if created {
		fmt.Println(
			"Created new node state:",
			dataPath,
		)
	} else {
		fmt.Println(
			"Loaded node state:",
			dataPath,
		)
	}

	if !chain.ValidateChain(pos) {
		fmt.Println(
			"Refusing reserved transfer execution from invalid local chain.",
		)
		return
	}

	governanceState, err :=
		chain.GetGovernanceState()

	if err != nil {
		fmt.Println(
			"Unable to reconstruct governance state:",
		)
		fmt.Println(err)
		return
	}

	pending, exists :=
		governanceState.GetPendingReservedTransferProposal(
			proposalID,
		)

	if !exists {
		fmt.Println(
			"Unknown pending reserved transfer proposal:",
			proposalID,
		)
		return
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	nextHeight :=
		lastBlock.Height + 1

	if err :=
		pending.RequireExecutableAt(
			nextHeight,
		); err != nil {

		fmt.Println(
			"Reserved transfer proposal is not executable yet:",
		)
		fmt.Println(err)

		fmt.Println(
			"Current height:",
			lastBlock.Height,
		)

		fmt.Println(
			"Next block height:",
			nextHeight,
		)

		fmt.Println(
			"Executable from height:",
			pending.ExecuteAfterHeight,
		)

		return
	}

	proposer, err :=
		pos.SelectProposer(
			lastBlock.Hash,
			nextHeight,
		)

	if err != nil {
		fmt.Println(
			"Unable to select proposer:",
			err,
		)
		return
	}

	execution :=
		reserved.NewReservedTransferExecution(
			proposalID,
		)

	block, err :=
		chain.AddReservedTransferExecutionBlock(
			[]reserved.ReservedTransferExecution{
				execution,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		fmt.Println(
			"Reserved transfer execution block rejected:",
		)
		fmt.Println(err)
		return
	}

	if err :=
		storage.Save(
			dataPath,
			chain,
			pos,
			wallets,
		); err != nil {

		fmt.Println(
			"Unable to save reserved transfer execution block:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== GOVERNED RESERVED TRANSFER EXECUTION CONFIRMED ===",
	)

	fmt.Println(
		"Proposal ID:",
		proposalID,
	)

	fmt.Println(
		"Pool:",
		pending.Proposal.Pool,
	)

	fmt.Println(
		"Recipient:",
		pending.Proposal.Recipient,
	)

	fmt.Println(
		"Amount:",
		pending.Proposal.Amount,
		"PRISM",
	)

	fmt.Println(
		"Proposal height:",
		pending.ProposalHeight,
	)

	fmt.Println(
		"Execution height:",
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

	if *peer != "" {
		if err :=
			broadcastGovernanceBlock(
				*port,
				*peer,
				dataPath,
				chain,
				pos,
				wallets,
				block,
			); err != nil {

			fmt.Println(
				"Reserved transfer block broadcast failed:",
			)
			fmt.Println(err)
			return
		}

		fmt.Println(
			"Block submission: ACCEPTED",
		)
	}
}

func addLocalReservedTransferApprovals(
	proposal *reserved.ReservedTransferProposal,
	policy reserved.AuthorityPolicy,
	pool consensus.ReservedPool,
	threshold uint32,
	wallets map[string]*wallet.Wallet,
) ([]string, error) {
	if proposal == nil {
		return nil,
			fmt.Errorf(
				"reserved transfer proposal cannot be nil",
			)
	}

	names :=
		make(
			[]string,
			0,
			len(wallets),
		)

	for name := range wallets {
		names =
			append(
				names,
				name,
			)
	}

	sort.Strings(names)

	approvedBy :=
		make(
			[]string,
			0,
			threshold,
		)

	for _, name := range names {
		currentWallet :=
			wallets[name]

		if currentWallet == nil {
			continue
		}

		authorized, err :=
			policy.IsAuthorized(
				pool,
				currentWallet.Address,
			)

		if err != nil {
			return nil, err
		}

		if !authorized {
			continue
		}

		if err :=
			proposal.AddApproval(
				currentWallet.Address,
				currentWallet.PublicKeyHex(),
				currentWallet.PrivateKey,
			); err != nil {

			return nil,
				fmt.Errorf(
					"cannot add reserved transfer approval from %s: %w",
					name,
					err,
				)
		}

		approvedBy =
			append(
				approvedBy,
				name,
			)

		if uint32(
			len(approvedBy),
		) >= threshold {

			break
		}
	}

	if uint32(
		len(approvedBy),
	) < threshold {

		return nil,
			fmt.Errorf(
				"not enough local authority keys: have %d need %d",
				len(approvedBy),
				threshold,
			)
	}

	return approvedBy, nil
}
