package node

import (
	"errors"
	"fmt"
	"path/filepath"

	"log/slog"
	"os"
	"statemachine/constants"
	"statemachine/replication"
	"statemachine/state"
	"statemachine/util"
	"sync"
)

type Node struct {
	mu sync.Mutex

	log replication.ReplicatedLog

	stateMachine *state.StateMachine

	commitIndex uint64
	lastApplied uint64
}

func CreateNode(root string, sm *state.StateMachine, logger *slog.Logger) (*Node, error) {

	if root == "" {
		return nil, fmt.Errorf("Node must have a root")
	}

	if err := os.MkdirAll(util.PathDir(root, constants.LogDirName), constants.DirectoryMode); err != nil {
		return nil, fmt.Errorf("Create Log directory: %w", err)
	}

	replicatedLogFileName := filepath.Join(util.PathDir(root, constants.LogDirName), constants.LogFileName)

	log, err := replication.CreateReplicatedLog(replicatedLogFileName, constants.LogFileMode)

	if err != nil {
		return nil, fmt.Errorf("Error opening replicated log: %w", err)
	}

	n := &Node{
		log:          log,
		stateMachine: sm,
		commitIndex:  log.LastIndex(),
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

func (n *Node) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	return errors.Join(n.log.Close(), n.stateMachine.Close())
}
