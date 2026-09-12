package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"prism/internal/blockchain"
	"prism/internal/p2p"
	"prism/internal/participation"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

type apiServer struct {
	dataPath string
	stateMu  sync.Mutex
}

type apiStatusResponse struct {
	Network     string `json:"network"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
	ChainID     string `json:"chainId"`
	Height      uint64 `json:"height"`
	Blocks      int    `json:"blocks"`
	Validators  int    `json:"validators"`
	TotalStake  uint64 `json:"totalStake"`
	TotalSupply uint64 `json:"totalSupply"`
	ChainValid  bool   `json:"chainValid"`
	GenesisHash string `json:"genesisHash"`
	LastHash    string `json:"lastHash"`
}

type apiHealthResponse struct {
	Status     string `json:"status"`
	Network    string `json:"network"`
	Version    string `json:"version"`
	Protocol   string `json:"protocol"`
	ChainID    string `json:"chainId"`
	Height     uint64 `json:"height"`
	ChainValid bool   `json:"chainValid"`
}

type apiValidatorResponse struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Stake            uint64 `json:"stake"`
	TotalBalance     uint64 `json:"totalBalance"`
	AvailableBalance uint64 `json:"availableBalance"`
}

type apiParticipationResponse struct {
	Name               string `json:"name"`
	Address            string `json:"address"`
	HumanityVerified   bool   `json:"humanityVerified"`
	BlocksProposed     uint64 `json:"blocksProposed"`
	UsefulWorkUnits    uint64 `json:"usefulWorkUnits"`
	ProposerScore      uint64 `json:"proposerScore"`
	UsefulWorkScore    uint64 `json:"usefulWorkScore"`
	ParticipationScore uint64 `json:"participationScore"`
}

type apiWorkResponse struct {
	Block         uint64 `json:"block"`
	Worker        string `json:"worker"`
	WorkerAddress string `json:"workerAddress"`
	Task          string `json:"task"`
	TaskID        string `json:"taskId"`
	Result        uint64 `json:"result"`
	Score         uint64 `json:"score"`
	Reward        uint64 `json:"reward"`
	Verified      bool   `json:"verified"`
	OutputHash    string `json:"outputHash"`
	ProofID       string `json:"proofId"`
	BlockHash     string `json:"blockHash"`
}

type apiHumanityResponse struct {
	Block         uint64 `json:"block"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	Provider      string `json:"provider"`
	Action        string `json:"action"`
	NullifierHash string `json:"nullifierHash"`
}

func runAPICommand(args []string) {
	flags := flag.NewFlagSet(
		"api",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

	host := flags.String(
		"host",
		"127.0.0.1",
		"HTTP host/interface used by the Prism API",
	)

	port := flags.Int(
		"port",
		8080,
		"HTTP port used by the Prism API",
	)

	nodeData := flags.String(
		"data",
		"data/node-7001",
		"Prism node data directory exposed by the API",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid API port:",
			*port,
		)
		return
	}

	if !storage.Exists(*nodeData) && !storage.ExistsPublic(*nodeData) {
		fmt.Println(
			"Prism API state not found:",
			*nodeData,
		)
		return
	}

	api := &apiServer{
		dataPath: *nodeData,
	}

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/api/v1/health",
		api.handleHealth,
	)

	mux.HandleFunc(
		"/api/v1/status",
		api.handleStatus,
	)

	mux.HandleFunc(
		"/api/v1/validators",
		api.handleValidators,
	)

	mux.HandleFunc(
		"/api/v1/participation",
		api.handleParticipation,
	)

	mux.HandleFunc(
		"/api/v1/work",
		api.handleWork,
	)

	mux.HandleFunc(
		"/api/v1/humanity",
		api.handleHumanity,
	)

	mux.HandleFunc(
		"/api/v1/reserved",
		api.handleReserved,
	)

	mux.HandleFunc(
		"/api/v1/mine/start",
		api.handleMineStart,
	)

	mux.HandleFunc(
		"/api/v1/mine/submit",
		api.handleMineSubmit,
	)
	listenAddress := fmt.Sprintf(
		"%s:%d",
		*host,
		*port,
	)

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           apiCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Println("=== PRISM HTTP API ===")
	fmt.Println("Version: 0.31")
	fmt.Println("P2P protocol:", p2p.ProtocolVersion)
	fmt.Println("Node data:", *nodeData)
	fmt.Println("Listening:", listenAddress)
	fmt.Println()

	fmt.Println("Endpoints:")
	fmt.Println("  GET /api/v1/health")
	fmt.Println("  GET /api/v1/status")
	fmt.Println("  GET /api/v1/validators")
	fmt.Println("  GET /api/v1/participation")
	fmt.Println("  GET /api/v1/work")
	fmt.Println("  GET /api/v1/humanity")
	fmt.Println("  GET /api/v1/reserved")
	fmt.Println("  POST /api/v1/mine/start")
	fmt.Println("  POST /api/v1/mine/submit")
	fmt.Println()

	fmt.Println(
		"Prism API running. Press Ctrl+C to stop.",
	)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println()
		fmt.Println(
			"Prism API stopped:",
			err,
		)
	}
}

func (api *apiServer) handleHealth(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, pos, _, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	if len(chain.Blocks) == 0 {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf("blockchain is empty"),
		)
		return
	}

	genesis := chain.Blocks[0]
	last := chain.Blocks[len(chain.Blocks)-1]

	response := apiHealthResponse{
		Status:     "ok",
		Network:    "Prism",
		Version:    "0.31",
		Protocol:   p2p.ProtocolVersion,
		ChainID:    p2p.MakeChainID(genesis.Hash),
		Height:     last.Height,
		ChainValid: chain.ValidateChain(pos),
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		response,
	)
}

func (api *apiServer) handleStatus(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, pos, _, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	if len(chain.Blocks) == 0 {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf("blockchain is empty"),
		)
		return
	}

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	genesis := chain.Blocks[0]
	last := chain.Blocks[len(chain.Blocks)-1]

	response := apiStatusResponse{
		Network:     "Prism",
		Version:     "0.31",
		Protocol:    p2p.ProtocolVersion,
		ChainID:     p2p.MakeChainID(genesis.Hash),
		Height:      last.Height,
		Blocks:      len(chain.Blocks),
		Validators:  len(pos.Validators),
		TotalStake:  pos.TotalStake(),
		TotalSupply: totalSupply,
		ChainValid:  chain.ValidateChain(pos),
		GenesisHash: genesis.Hash,
		LastHash:    last.Hash,
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		response,
	)
}

func (api *apiServer) handleValidators(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, pos, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	validators := make(
		[]apiValidatorResponse,
		0,
		len(pos.Validators),
	)

	for _, validator := range pos.Validators {
		totalBalance, err := chain.BalanceOf(
			validator.Address,
		)
		if err != nil {
			apiWriteError(
				writer,
				http.StatusInternalServerError,
				err,
			)
			return
		}

		availableBalance, err :=
			chain.AvailableBalanceOf(
				validator.Address,
			)
		if err != nil {
			apiWriteError(
				writer,
				http.StatusInternalServerError,
				err,
			)
			return
		}

		validators = append(
			validators,
			apiValidatorResponse{
				Name: walletNameForAddress(
					validator.Address,
					wallets,
				),
				Address:          validator.Address,
				Stake:            validator.Stake,
				TotalBalance:     totalBalance,
				AvailableBalance: availableBalance,
			},
		)
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"validators": validators,
			"totalStake": pos.TotalStake(),
		},
	)
}

func (api *apiServer) handleParticipation(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, pos, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	scores, err := participation.Calculate(
		chain,
		pos,
		chain,
	)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	results := make(
		[]apiParticipationResponse,
		0,
		len(scores),
	)

	for _, score := range scores {
		results = append(
			results,
			apiParticipationResponse{
				Name: walletNameForAddress(
					score.Address,
					wallets,
				),
				Address: score.Address,
				HumanityVerified: chain.IsVerified(
					score.Address,
				),
				BlocksProposed:     score.BlocksProposed,
				UsefulWorkUnits:    score.UsefulWorkUnits,
				ProposerScore:      score.ProposerScore,
				UsefulWorkScore:    score.UsefulWorkScore,
				ParticipationScore: score.ParticipationScore,
			},
		)
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"humanityRequired": true,
			"participants":     results,
		},
	)
}

func (api *apiServer) handleWork(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, _, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	results := make(
		[]apiWorkResponse,
		0,
	)

	for blockIndex := len(chain.Blocks) - 1; blockIndex >= 0; blockIndex-- {
		block := chain.Blocks[blockIndex]

		for proofIndex := len(block.UsefulWork) - 1; proofIndex >= 0; proofIndex-- {
			proof := block.UsefulWork[proofIndex]

			verifyErr := usefulwork.VerifyProof(
				proof,
			)

			results = append(
				results,
				apiWorkResponse{
					Block: block.Height,
					Worker: walletNameForAddress(
						proof.Worker,
						wallets,
					),
					WorkerAddress: proof.Worker,
					Task:          proof.Task.Type,
					TaskID:        proof.Task.ID,
					Result:        proof.Result,
					Score:         proof.Score,
					Reward:        blockchain.UsefulWorkReward,
					Verified:      verifyErr == nil,
					OutputHash:    proof.OutputHash,
					ProofID:       proof.ID,
					BlockHash:     block.Hash,
				},
			)
		}
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"count":   len(results),
			"entries": results,
		},
	)
}

func (api *apiServer) handleHumanity(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, _, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	results := make(
		[]apiHumanityResponse,
		0,
	)

	seen := make(
		map[string]struct{},
	)

	for _, block := range chain.Blocks {
		for _, attestation := range block.Humanity {
			if _, exists := seen[attestation.Address]; exists {
				continue
			}

			seen[attestation.Address] = struct{}{}

			results = append(
				results,
				apiHumanityResponse{
					Block: block.Height,
					Name: walletNameForAddress(
						attestation.Address,
						wallets,
					),
					Address:       attestation.Address,
					Provider:      attestation.Provider,
					Action:        attestation.Action,
					NullifierHash: attestation.NullifierHash,
				},
			)
		}
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"count":      len(results),
			"identities": results,
		},
	)
}
func apiGETOnly(
	writer http.ResponseWriter,
	request *http.Request,
) bool {
	if request.Method == http.MethodGet {
		return true
	}

	writer.Header().Set(
		"Allow",
		http.MethodGet,
	)

	apiWriteJSON(
		writer,
		http.StatusMethodNotAllowed,
		map[string]string{
			"error": "method not allowed",
		},
	)

	return false
}

func apiWriteError(
	writer http.ResponseWriter,
	status int,
	err error,
) {
	apiWriteJSON(
		writer,
		status,
		map[string]string{
			"error": err.Error(),
		},
	)
}

func apiWriteJSON(
	writer http.ResponseWriter,
	status int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)

	writer.WriteHeader(status)

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(value); err != nil {
		fmt.Println(
			"Unable to encode API response:",
			err,
		)
	}
}

func apiCORS(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set(
				"Access-Control-Allow-Origin",
				"*",
			)

			writer.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, OPTIONS",
			)

			writer.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type",
			)

			if request.Method == http.MethodOptions {
				writer.WriteHeader(
					http.StatusNoContent,
				)
				return
			}

			next.ServeHTTP(
				writer,
				request,
			)
		},
	)
}
