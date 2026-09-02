package replication

import "statemachine/types"

type ReplicatedLog interface {
	Append(data []byte) (types.LogEntry, error)
	Get(index uint64) (types.LogEntry, error)

	FirstIndex() uint64
	LastIndex() uint64

	Close() error
}

type Reader interface {
	Read(location types.Location) ([]byte, error)
}

type Writer interface {
	Append(data []byte) (types.Location, error)
	Close() error
}
