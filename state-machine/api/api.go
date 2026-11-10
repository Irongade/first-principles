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
	"strings"
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
	mux.HandleFunc("/health", e.handleHealth)
	mux.HandleFunc("/node/", e.handleMethods)
	return mux
}

func (e *Engine) handleHealth(res http.ResponseWriter, _ *http.Request) {
	writeJson(res, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (e *Engine) handleMethods(res http.ResponseWriter, req *http.Request) {
	key := strings.TrimPrefix(req.URL.Path, "/node/")

	if key == "" || strings.Contains(key, "/") {
		writeError(res, http.StatusBadRequest, "invalid key provided")
		return
	}

	switch req.Method {
	case http.MethodGet:
		e.handleGet(res, req, key)
	case http.MethodPut:
		var payload struct {
			Value *string `json:"value"`
		}

		body := http.MaxBytesReader(res, req.Body, 1<<20)

		if err := json.NewDecoder(body).Decode(&payload); err != nil || payload.Value == nil {
			writeError(res, http.StatusBadRequest, `body must be {"value": "..."}`)
			return
		}

		e.propose(res, command.Command{Operation: constants.PUT_OPERATION, Key: key, Value: *payload.Value})

	case http.MethodDelete:
		e.propose(res, command.Command{Operation: constants.DELETE_OPERATION, Key: key})

	default:
		res.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(res, http.StatusMethodNotAllowed, "Method not allowed")
	}

}

func (e *Engine) handleGet(res http.ResponseWriter, _ *http.Request, key string) {

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
