package node

import (
	"errors"
	"fmt"
	"path/filepath"

	"log/slog"
	"os"
	"statemachine/constants"
	"statemachine/replication"
	"statemachine/role"
	"statemachine/state"
	"statemachine/transport"
	"statemachine/util"
	"sync"
)

type Config struct {
	ID    string
	Root  string
	Peers []string
	Role  role.Role
}

type Node struct {
	mu sync.Mutex

	log          replication.ReplicatedLog
	stateMachine *state.StateMachine

	commitIndex uint64
	lastApplied uint64

	id        string
	peers     []string
	role      role.Role
	nextIndex map[string]uint64

	client transport.Transport
}

func CreateNode(config Config, sm *state.StateMachine, tr transport.Transport, logger *slog.Logger) (*Node, error) {

	if config.Root == "" {
		return nil, fmt.Errorf("Node must have a root")
	}

	if err := os.MkdirAll(util.PathDir(config.Root, constants.LogDirName), constants.DirectoryMode); err != nil {
		return nil, fmt.Errorf("Create Log directory: %w", err)
	}

	replicatedLogFileName := filepath.Join(util.PathDir(config.Root, constants.LogDirName), constants.LogFileName)

	log, err := replication.CreateReplicatedLog(replicatedLogFileName, constants.LogFileMode)

	if err != nil {
		return nil, fmt.Errorf("Error opening replicated log: %w", err)
	}

	n := &Node{
		log:          log,
		stateMachine: sm,
		commitIndex:  log.LastIndex(),

		client:    tr,
		id:        config.ID,
		role:      config.Role,
		peers:     config.Peers,
		nextIndex: make(map[string]uint64, len(config.Peers)),
	}

	// apply existing commit indexes.
	n.mu.Lock()
	defer n.mu.Unlock()

	if err := n.applyCommitted(); err != nil {
		log.Close()
		return nil, fmt.Errorf("Error replaying index on startup: %w", err)
	}

	logger.Info("Node has started", "log_file", replicatedLogFileName, "last_index", log.LastIndex())

	return n, nil

}

func (n *Node) Propose(data []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	entry, err := n.log.Append(data)

	if err != nil {
		return err
	}

	// once the entry has been committed, we update the commit index
	n.commitIndex = entry.Index

	return n.applyCommitted()
}

func (n *Node) applyCommitted() error {
	for n.lastApplied < n.commitIndex {
		nextIndex := n.lastApplied + 1

		entry, err := n.log.Read(nextIndex)

		if err != nil {
			return err
		}

		if err := n.stateMachine.Apply(entry.Data); err != nil {
			return err
		}

		n.lastApplied = nextIndex
	}

	return nil
}

func (n *Node) HandleAppendEntries(req transport.AppendEntriesRequest) (transport.AppendEntriesResponse, error) {

	return transport.AppendEntriesResponse{
		Success:   false,
		LastIndex: 0,
	}, nil
}

func (n *Node) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	return errors.Join(n.log.Close(), n.stateMachine.Close())
}
