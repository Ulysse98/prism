package main

import (
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/p2p"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func runNodeGovernanceProposeCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node-governance-propose",
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
			`prism node-governance-propose --port 7001 --peer 127.0.0.1:7002 treasury add <address> 1`,
		)
		fmt.Println()
		fmt.Println(
			"Arguments: <pool> <add|remove> <authority> <nonce>",
		)
		return
	}

	pool, err :=
		parseGovernancePool(
			positional[0],
		)

	if err != nil {
		fmt.Println(
			"Invalid governance pool:",
			err,
		)
		return
	}

	action, err :=
		parseGovernanceAction(
			positional[1],
		)

	if err != nil {
		fmt.Println(
			"Invalid governance action:",
			err,
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
			"Governance nonce must be greater than zero.",
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
			"Refusing governance proposal from invalid local chain.",
		)
		return
	}

	targetAddress, targetLabel, err :=
		resolveAddress(
			positional[2],
			wallets,
		)

	if err != nil {
		fmt.Println(
			"Invalid governance authority target:",
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
		reserved.NewAuthorityProposal(
			chainID,
			nonce,
			pool,
			action,
			targetAddress,
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
			"Unable to determine governance threshold:",
			err,
		)
		return
	}

	approvedBy, err :=
		addLocalGovernanceApprovals(
			&proposal,
			governanceState.CurrentPolicy,
			pool,
			threshold,
			wallets,
		)

	if err != nil {
		fmt.Println(
			"Unable to authorize governance proposal:",
		)
		fmt.Println(err)
		return
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	proposer, err :=
		pos.SelectProposer(
			lastBlock.Hash,
			lastBlock.Height+1,
		)

	if err != nil {
		fmt.Println(
			"Unable to select proposer:",
			err,
		)
		return
	}

	block, err :=
		chain.AddAuthorityProposalBlock(
			[]reserved.AuthorityProposal{
				proposal,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		fmt.Println(
			"Governance proposal block rejected:",
		)
		fmt.Println(err)
		return
	}

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {

		fmt.Println(
			"Unable to save governance proposal block:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== QUEUED GOVERNANCE PROPOSAL CONFIRMED ===",
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
		"Action:",
		action,
	)
	fmt.Println(
		"Authority:",
		targetLabel,
	)
	fmt.Println(
		"Authority address:",
		targetAddress,
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
				"Governance block broadcast failed:",
			)
			fmt.Println(err)
			return
		}

		fmt.Println(
			"Block submission: ACCEPTED",
		)
	}
}

func runNodeGovernanceExecuteCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node-governance-execute",
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
			`prism node-governance-execute --port 7001 --peer 127.0.0.1:7002 <proposal-id>`,
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
			"Refusing governance execution from invalid local chain.",
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
		governanceState.GetPendingAuthorityProposal(
			proposalID,
		)

	if !exists {
		fmt.Println(
			"Unknown pending governance proposal:",
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
			"Governance proposal is not executable yet:",
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
		reserved.NewAuthorityExecution(
			proposalID,
		)

	block, err :=
		chain.AddAuthorityExecutionBlock(
			[]reserved.AuthorityExecution{
				execution,
			},
			proposer.Address,
			pos,
		)

	if err != nil {
		fmt.Println(
			"Governance execution block rejected:",
		)
		fmt.Println(err)
		return
	}

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {

		fmt.Println(
			"Unable to save governance execution block:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== QUEUED GOVERNANCE EXECUTION CONFIRMED ===",
	)

	fmt.Println(
		"Proposal ID:",
		proposalID,
	)

	fmt.Println(
		"Pool:",
		pending.Proposal.Change.Pool,
	)

	fmt.Println(
		"Action:",
		pending.Proposal.Change.Action,
	)

	fmt.Println(
		"Authority:",
		pending.Proposal.Change.Authority,
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
				"Governance block broadcast failed:",
			)
			fmt.Println(err)
			return
		}

		fmt.Println(
			"Block submission: ACCEPTED",
		)
	}
}

func parseGovernancePool(
	value string,
) (consensus.ReservedPool, error) {
	switch strings.ToLower(
		strings.TrimSpace(value),
	) {
	case "ecosystem":
		return consensus.ReservedPoolEcosystem,
			nil

	case "treasury":
		return consensus.ReservedPoolTreasury,
			nil

	case "team":
		return consensus.ReservedPoolTeam,
			nil

	case "liquidity":
		return consensus.ReservedPoolLiquidity,
			nil

	default:
		return "",
			fmt.Errorf(
				"expected ecosystem, treasury, team, or liquidity",
			)
	}
}

func parseGovernanceAction(
	value string,
) (reserved.AuthorityChangeAction, error) {
	switch strings.ToLower(
		strings.TrimSpace(value),
	) {
	case "add":
		return reserved.AuthorityChangeAdd,
			nil

	case "remove":
		return reserved.AuthorityChangeRemove,
			nil

	default:
		return "",
			fmt.Errorf(
				"expected add or remove",
			)
	}
}

func addLocalGovernanceApprovals(
	proposal *reserved.AuthorityProposal,
	policy reserved.AuthorityPolicy,
	pool consensus.ReservedPool,
	threshold uint32,
	wallets map[string]*wallet.Wallet,
) ([]string, error) {
	if proposal == nil {
		return nil,
			fmt.Errorf(
				"authority proposal cannot be nil",
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

		if err := proposal.AddApproval(
			currentWallet.Address,
			currentWallet.PublicKeyHex(),
			currentWallet.PrivateKey,
		); err != nil {

			return nil,
				fmt.Errorf(
					"cannot add approval from %s: %w",
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

func broadcastGovernanceBlock(
	port int,
	peer string,
	dataPath string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
	block blockchain.Block,
) error {
	identity :=
		wallets["Alice"]

	if identity == nil {
		return fmt.Errorf(
			"node identity wallet is missing",
		)
	}

	server :=
		p2p.NewServer(
			p2p.MakeNodeID(
				identity.Address,
			),
			fmt.Sprintf(
				"0.0.0.0:%d",
				port,
			),
			dataPath,
			chain,
			pos,
			wallets,
		)

	fmt.Println()
	fmt.Println(
		"Broadcasting governance block to:",
		peer,
	)

	return server.SendBlock(
		peer,
		block,
	)
}
