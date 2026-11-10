package replication

import "statemachine/types"

type ReplicatedLog interface {
	Append(data []byte) (types.LogEntry, error)
	Read(index uint64) (types.LogEntry, error)

	FirstIndex() uint64
	LastIndex() uint64

	Close() error
}

type Reader interface {
	Read(location types.Location) ([]byte, error)
	RebuildIndex() (map[uint64]types.Location, uint64, uint64, error)
	Close() error
}

type Writer interface {
	Append(data []byte) (types.Location, error)
	Close() error
}
