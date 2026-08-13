package api

import (
	"encoding/json"
	"errors"
	"kvstore/variables"
	"log/slog"
	"net/http"
	"strings"
)

const maxRequestBodySize = 1 << 20 // 1 MB

type Store interface {
	// get retrieves a string value or returns an error
	Get(K string) (string, error)

	// Put updates a value or returns an error
	Put(K string, V string) error

	// Delete returns an error
	Delete(K string) error

	// closes the DB
	Close() error

	// manual compaction of segments
	CompactSegments() error
}

type RequestPayload struct {
	Value *string `json:"value"`
}

type Engine struct {
	store  Store
	logger *slog.Logger
}

func NewEngine(store Store, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}

	return &Engine{
		store:  store,
		logger: logger,
	}
}

func (e *Engine) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", e.handleHealth)
	mux.HandleFunc("/kv/", e.handleMethods)
	mux.HandleFunc("/compaction", e.handleCompaction)

	return e.loggingMiddleware(mux)
}

func (e *Engine) handleMethods(res http.ResponseWriter, req *http.Request) {
	key := strings.TrimPrefix(req.URL.Path, "/kv/")

	if key == "" || strings.Contains(key, "/") {
		writeError(res, http.StatusBadRequest, "invalid key provided")
		return
	}

	switch req.Method {
	case http.MethodGet:
		e.handleGet(res, req, key)
	case http.MethodPut:
		e.handlePut(res, req, key)
	case http.MethodDelete:
		e.handleDelete(res, req, key)

	default:
		res.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(res, http.StatusMethodNotAllowed, "Method not allowed")
	}

}

func (e *Engine) handleGet(res http.ResponseWriter, _ *http.Request, key string) {

	value, err := e.store.Get(key)

	if err != nil {
		if errors.Is(err, variables.KeyNotFound) {
			writeError(res, http.StatusNotFound, variables.KeyNotFound.Error())
			return
		}

		e.logger.Error("Failed to get key: %w, error: %w", key, err)
		writeError(res, http.StatusInternalServerError, "Failed to fetch value")

		return
	}

	writeJson(res, http.StatusOK, map[string]string{
		"key":   key,
		"value": value,
	})
}

func (e *Engine) handlePut(res http.ResponseWriter, req *http.Request, key string) {
	req.Body = http.MaxBytesReader(res, req.Body, maxRequestBodySize)
	defer req.Body.Close()

	var request RequestPayload

	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeError(res, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if request.Value == nil {
		writeError(res, http.StatusBadRequest, "Value is required")
		return
	}

	e.logger.Info("put request decoded", "key", key, "value", *request.Value)

	if err := e.store.Put(key, *request.Value); err != nil {
		e.logger.Error("Put Request failed for key", key, "value", *request.Value, "err", err.Error(), ".")
		writeError(res, http.StatusBadRequest, "Failed to store value")
		return
	}

	e.logger.Info("put succeeded", "key", key)

	writeJson(res, http.StatusCreated, map[string]any{
		"key":   key,
		"value": *request.Value,
	})
}

func (e *Engine) handleDelete(res http.ResponseWriter, _ *http.Request, key string) {

	err := e.store.Delete(key)

	if err != nil {
		if errors.Is(err, variables.KeyNotFound) {
			writeError(res, http.StatusNotFound, variables.KeyNotFound.Error())
			return
		}

		e.logger.Error("failed to delete key",
			"key", key,
			"error", err,
		)
		writeError(res, http.StatusInternalServerError, "Failed to fetch value")

		return
	}

	res.WriteHeader(http.StatusNoContent)

}

func (e *Engine) handleHealth(res http.ResponseWriter, _ *http.Request) {
	writeJson(res, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (e *Engine) handleCompaction(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		err := e.store.CompactSegments()

		if err != nil {
			writeError(res, http.StatusBadRequest, err.Error())
			return
		}

		writeJson(res, http.StatusOK, map[string]string{
			"status": "ok",
		})

	default:
		writeError(res, http.StatusBadRequest, "Incorrect HTTP verb")
	}

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

func (e *Engine) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		e.logger.Info(
			"http request",
			"method", req.Method,
			"path", req.URL.Path,
			"remote_address", req.RemoteAddr,
		)

		next.ServeHTTP(res, req)
	})
}
