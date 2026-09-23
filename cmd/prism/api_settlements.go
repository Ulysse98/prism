package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"prism/internal/crosschain"
)

func settlementWriteAllowed(
	request *http.Request,
) bool {
	remote :=
		strings.TrimSpace(
			request.RemoteAddr,
		)

	host, _, err :=
		net.SplitHostPort(remote)

	if err != nil {
		host = remote
	}

	ip :=
		net.ParseIP(host)

	return ip != nil &&
		ip.IsLoopback()
}

func (api *apiServer) handleSettlements(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if api.settlements == nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf(
				"cross-chain settlement store is unavailable",
			),
		)
		return
	}

	if request.Method != http.MethodPost {
		writer.Header().Set(
			"Allow",
			http.MethodPost,
		)

		apiWriteError(
			writer,
			http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"),
		)
		return
	}

	if !settlementWriteAllowed(
		request,
	) {
		apiWriteError(
			writer,
			http.StatusForbidden,
			fmt.Errorf(
				"settlement writes are restricted to the local recorder",
			),
		)
		return
	}

	request.Body =
		http.MaxBytesReader(
			writer,
			request.Body,
			16384,
		)

	var payload crosschain.Settlement

	decoder :=
		json.NewDecoder(
			request.Body,
		)

	decoder.DisallowUnknownFields()

	if err := decoder.Decode(
		&payload,
	); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid settlement request: %w",
				err,
			),
		)
		return
	}

	settlement, err :=
		api.settlements.Upsert(
			payload,
		)

	if err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			err,
		)
		return
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"settlement": settlement,
		},
	)
}

func (api *apiServer) handleSettlementByRegistry(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if api.settlements == nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf(
				"cross-chain settlement store is unavailable",
			),
		)
		return
	}

	if request.Method != http.MethodGet {
		writer.Header().Set(
			"Allow",
			http.MethodGet,
		)

		apiWriteError(
			writer,
			http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"),
		)
		return
	}

	registryID :=
		strings.TrimPrefix(
			request.URL.Path,
			"/api/v1/settlements/",
		)

	registryID =
		strings.ToLower(
			strings.TrimSpace(
				registryID,
			),
		)

	if registryID == "" ||
		strings.Contains(
			registryID,
			"/",
		) {

		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid settlement registry ID",
			),
		)
		return
	}

	settlements, err :=
		api.settlements.ForRegistry(
			registryID,
		)

	if err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			err,
		)
		return
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"registryId":  registryID,
			"settlements": settlements,
		},
	)
}
