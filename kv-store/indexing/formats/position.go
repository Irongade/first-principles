package formats

import (
	"fmt"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	"kvstore/indexing"
)

type PositionIndexer struct {
	IndexFormat indexing.IndexFormat
	Reader      *filereader.FileReader
	index       map[string]indexing.IndexEntry
}

func (bi *PositionIndexer) Populate() error {

	// load up all the logs in memory in our index, for fast gets and updates.
	records, err := bi.Reader.ScanAll()

	if err != nil {
		return fmt.Errorf("Error populating index: %w", err)
	}

	bi.index = make(map[string]indexing.IndexEntry, len(records))

	for _, record := range records {

		if record.Operation == constants.DELETE_OPERATION {
			bi.Delete(record.Key)
		} else {
			entry := indexing.IndexEntry{
				Value:          record.Value,
				RecordLocation: record.RecordLocation,
			}

			bi.index[record.Key] = entry
		}

	}

	return nil
}

func (bi *PositionIndexer) Get(K string) (indexing.IndexEntry, bool) {
	val, ok := bi.index[K]

	if !ok {
		return indexing.IndexEntry{}, false
	}

	return val, true
}

func (bi *PositionIndexer) Update(K string, entry indexing.IndexEntry) error {
	bi.index[K] = entry

	return nil
}

func (bi *PositionIndexer) Delete(K string) error {
	delete(bi.index, K)

	return nil
}
