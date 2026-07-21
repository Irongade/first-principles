package indexing

import "kvstore/types"

type IndexFormat string

const (
	BasicIndex IndexFormat = "basic"
)

type IndexEntry struct {
	Value string
}

type Indexer interface {
	Populate(records []types.Record) error
	Update(key string, entry IndexEntry) error
	Get(key string) (IndexEntry, bool)
	Delete(key string) error
}
