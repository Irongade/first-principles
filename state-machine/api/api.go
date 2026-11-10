package api

import (
	"encoding/json"
	"errors"
	"kvstore/variables"
	"log/slog"
	"net/http"
	"statemachine/command"
	"statemachine/constants"
	"statemachine/node"
	"statemachine/state"
	"statemachine/transport"
)

type Engine struct {
	node *node.Node
	sm   *state.StateMachine

	logger *slog.Logger
}

func NewEngine(n *node.Node, sm *state.StateMachine, logger *slog.Logger) *Engine {
	return &Engine{
		node:   n,
		sm:     sm,
		logger: logger,
	}
}

func (e *Engine) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", e.handleHealth)
	mux.HandleFunc("GET /node/{key}", e.handleGet)
	mux.HandleFunc("PUT /node/{key}", e.handlePut)
	mux.HandleFunc("DELETE /node/{key}", e.handleDelete)
	mux.HandleFunc("POST /internal/append-entries", e.handleAppendEntries)

	return mux
}

func (e *Engine) handleHealth(res http.ResponseWriter, _ *http.Request) {
	writeJson(res, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (e *Engine) handleGet(res http.ResponseWriter, req *http.Request) {
	key := req.PathValue("key")

	value, err := e.sm.Get(key)

	if err != nil {
		if errors.Is(err, variables.KeyNotFound) {
			writeError(res, http.StatusNotFound, variables.KeyNotFound.Error())
			return
		}

		e.logger.Error("get failed", "key", key, "error", err)
		writeError(res, http.StatusInternalServerError, "Failed to fetch value")

		return
	}

	writeJson(res, http.StatusOK, map[string]string{
		"key":   key,
		"value": value,
	})
}

func (e *Engine) handlePut(res http.ResponseWriter, req *http.Request) {
	key := req.PathValue("key")

	var payload struct {
		Value *string `json:"value"`
	}

	body := http.MaxBytesReader(res, req.Body, 1<<20)

	if err := json.NewDecoder(body).Decode(&payload); err != nil || payload.Value == nil {
		writeError(res, http.StatusBadRequest, `body must be {"value": "..."}`)
		return
	}

	e.propose(res, command.Command{Operation: constants.PUT_OPERATION, Key: key, Value: *payload.Value})

}

func (e *Engine) handleDelete(res http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	e.propose(res, command.Command{Operation: constants.DELETE_OPERATION, Key: key})
}

func (e *Engine) handleAppendEntries(res http.ResponseWriter, r *http.Request) {
	var req transport.AppendEntriesRequest

	body := http.MaxBytesReader(res, r.Body, 4<<20)

	if err := json.NewDecoder(body).Decode(&req); err != nil {
		writeError(res, http.StatusBadRequest, "Invalid request body sent")

		return
	}

	resp, err := e.node.HandleAppendEntries(req)

	if err != nil {
		e.logger.Error("Failed to append entries from leader", "error", err)

		writeError(res, http.StatusInternalServerError, "append entries failed")
		return
	}

	writeJson(res, http.StatusOK, resp)
}

func (e *Engine) propose(w http.ResponseWriter, cmd command.Command) {
	if err := cmd.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := command.EncodeCommand(cmd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode command failed")
		return
	}

	if err := e.node.Propose(data); err != nil {
		e.logger.Error("propose failed", "error", err)
		writeError(w, http.StatusInternalServerError, "propose failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJson(res http.ResponseWriter, status int, value any) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)

	if err := json.NewEncoder(res).Encode(value); err != nil {
		slog.Error("Write json response failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJson(w, status, map[string]string{
		"error": message,
	})
}
