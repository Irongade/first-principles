package formats

import (
	"fmt"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	"kvstore/indexing"
	"kvstore/types"
)

type ValueIndexer struct {
	IndexFormat types.IndexFormat
	Reader      filereader.FileReader
	index       map[string]indexing.IndexEntry
}

func (bi *ValueIndexer) Populate() error {
	// load up all the logs in memory in our index, for fast gets and updates.
	records, err := bi.Reader.ReadAll()

	if err != nil {
		return fmt.Errorf("Error populating index: %w", err)
	}

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

func (bi *ValueIndexer) Get(K string) (indexing.IndexEntry, bool) {
	val, ok := bi.index[K]

	if !ok {
		return indexing.IndexEntry{}, false
	}

	return val, true
}

func (bi *ValueIndexer) Update(K string, entry indexing.IndexEntry) error {
	bi.index[K] = entry

	return nil
}

func (bi *ValueIndexer) Delete(K string) error {
	delete(bi.index, K)

	return nil
}

func (bi *ValueIndexer) View() error {
	fmt.Println(bi.index)

	return nil
}
