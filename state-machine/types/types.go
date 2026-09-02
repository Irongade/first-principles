package types

type Operation string

type Store interface {
	Get(K string) (string, error)
	Put(K string, V string) error
	Delete(K string) error
	Close() error
}

type Location struct {
	Offset int64
	Size   uint32
}

type LogEntry struct {
	Index uint64
	Term  uint64
	Data  []byte
}
