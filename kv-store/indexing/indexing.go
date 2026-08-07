package indexing

import (
	"kvstore/types"
)

type IndexFormat string

const (
	BasicIndex    IndexFormat = "basic"
	PositionIndex IndexFormat = "position"
)

type IndexEntry struct {
	Value          string
	RecordLocation types.RecordLocation
}

type Indexer interface {
	Populate() error
	Update(key string, entry IndexEntry) error
	Get(key string) (IndexEntry, bool)
	Delete(key string) error
	View() error
}
