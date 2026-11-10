package transport

import (
	"context"
	"statemachine/types"
)

type AppendEntriesRequest struct {
	LeaderID     string
	Entries      []types.LogEntry
	LeaderCommit uint64
}

type AppendEntriesResponse struct {
	Success   bool
	LastIndex uint64
}

type Transport interface {
	SendAppendEntries(ctx context.Context, peerId string, req AppendEntriesRequest) (AppendEntriesResponse, error)
}
