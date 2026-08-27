package state

import (
	"fmt"
	"log/slog"
	"os"
	"statemachine/constants"
	"statemachine/types"
	"sync"

	"kvstore/config"
	"kvstore/storage"
)

type Command struct {
	Operation types.Operation
	Key       string
	Value     string
}

type StateMachine struct {
	store types.Store
	mu    sync.Mutex
}

func CreateNewStateMachine() (*StateMachine, error) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := config.Load(logger)
	cfg.LogSummary(logger)
	cfg.Validate(logger)

	store, err := storage.NewFileStore(cfg)

	if err != nil {
		logger.Error("Failed to create store for state machine", "error", err)

		return nil, err
	}

	return &StateMachine{
		store: store,
	}, nil
}

func (sm *StateMachine) Get(K string) (string, error) {
	value, err := sm.store.Get(K)

	if err != nil {
		return "", err
	}

	return value, nil
}

func (sm *StateMachine) Close() error {
	return sm.store.Close()
}

func (sm *StateMachine) Apply(cmd Command) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch cmd.Operation {
	case constants.PUT_OPERATION:
		return sm.store.Put(cmd.Key, cmd.Value)

	case constants.DELETE_OPERATION:
		return sm.store.Delete(cmd.Key)

	default:
		return fmt.Errorf("Unsupported State Machine operation: %s", cmd.Operation)
	}
}
