package formats

import (
	"kvstore/constants"
	"kvstore/indexing"
	"kvstore/types"
)

type BasicIndexer struct {
	IndexFormat indexing.IndexFormat
	index       map[string]indexing.IndexEntry
}

func (bi *BasicIndexer) Populate(records []types.Record) error {
	bi.index = make(map[string]indexing.IndexEntry, len(records))

	for _, record := range records {

		if record.Operation == constants.DELETE_OPERATION {
			bi.Delete(record.Key)
		} else {
			entry := indexing.IndexEntry{
				Value: record.Value,
			}

			bi.index[record.Key] = entry
		}

	}

	return nil
}

func (bi *BasicIndexer) Get(K string) (indexing.IndexEntry, bool) {
	val, ok := bi.index[K]

	if !ok {
		return indexing.IndexEntry{}, false
	}

	return val, true
}

func (bi *BasicIndexer) Update(K string, entry indexing.IndexEntry) error {
	bi.index[K] = entry

	return nil
}

func (bi *BasicIndexer) Delete(K string) error {
	delete(bi.index, K)

	return nil
}
