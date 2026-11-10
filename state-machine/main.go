package main

import (
	"context"
	"errors"
	"fmt"
	"kvstore/config"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"statemachine/api"
	"statemachine/constants"
	"statemachine/node"
	"statemachine/role"
	"statemachine/state"
	httptransport "statemachine/transport/http-transport"
	"statemachine/util"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	root := envOr("NODE_DATA_DIR", "data/node-1")
	addr := envOr("NODE_ADDR", ":8081")
	driver := envOr("STATE_MACHINE_DRIVER", "kv")

	nodeID := envOr("NODE_ID", "1")
	leaderID := envOr("NODE_LEADER", "1")
	nodePeers := envOr("NODE_PEERS", "1=http://localhost:8081")

	peers, err := util.ParsePeers(nodePeers, nodeID)

	if err != nil {
		logger.Error("Error parsing peers: %w", err)
		os.Exit(1)
	}

	client, err := httptransport.CreateNewHttpTransport(peers, 2*time.Second)

	if err != nil {
		logger.Error("Error creating http transport for this node. Node ID: %s, error: %w", nodeID, err)
		os.Exit(1)
	}

	peerIDs := make([]string, len(peers))

	for id := range peers {
		peerIDs = append(peerIDs, id)
	}

	sort.Strings(peerIDs)

	sm, err := buildStateMachine(driver, root, logger)
	if err != nil {
		logger.Error("build state machine", "error", err)
		os.Exit(1)
	}

	r := role.Follower

	if nodeID == leaderID {
		r = role.Leader
	}

	n, err := node.CreateNode(node.Config{
		ID:    nodeID,
		Role:  r,
		Root:  root,
		Peers: peerIDs,
	}, sm, client, logger)
	if err != nil {
		sm.Close()
		logger.Error("create node", "error", err)
		os.Exit(1)
	}

	// API stuff.

	engine := api.NewEngine(n, sm, logger)

	server := &http.Server{
		Addr:              addr,
		Handler:           engine.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("Server started", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case signal := <-signals:
		logger.Info("shutdown signal received", "signal", signal)

	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}

	if err := n.Close(); err != nil {
		logger.Error("store close failed", "error", err)
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func buildStateMachine(driver, root string, logger *slog.Logger) (*state.StateMachine, error) {
	switch driver {
	case "kv":
		cfg := config.Load(logger)
		cfg.DataDir = util.PathDir(root, constants.DbDirName)
		cfg.LogSummary(logger)
		cfg.Validate(logger)
		return state.CreateNewStateMachine(cfg, logger)
	default:
		return nil, fmt.Errorf("unknown state machine driver %q", driver)
	}
}
