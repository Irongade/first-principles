package storage

import (
	"errors"
	"fmt"
	"kvstore/compaction"
	"kvstore/config"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	segmentreader "kvstore/file-reader/segment"
	simplereader "kvstore/file-reader/simple"
	fileutil "kvstore/file-util"
	filewriter "kvstore/file-writer"
	segmentwriter "kvstore/file-writer/segment"
	simplewriter "kvstore/file-writer/simple"
	"kvstore/indexing"
	"kvstore/indexing/formats"
	"kvstore/types"
	"kvstore/variables"
	"os"
	"path"
	"sync"
	"time"
)

const FILE_PATH_PREFIX = "./data/"

type FileStore struct {
	filename    string
	fileWriter  filewriter.FileWriter
	fileReader  filereader.FileReader
	indexer     indexing.Indexer
	indexFormat types.IndexFormat
	compactor   *compaction.Compactor

	mu      sync.RWMutex
	version string
	closed  bool

	stopCompaction chan struct{}
	compactionWg   sync.WaitGroup
	stopOnce       sync.Once
}

func NewFileStore(config config.Config) (*FileStore, error) {
	filepath := FILE_PATH_PREFIX + config.FileName

	dir := path.Dir(filepath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	var newFileWriter filewriter.FileWriter
	var err error

	switch config.StorageFormat {
	case constants.SegmentFileFormat:
		newFileWriter, err = segmentwriter.CreateFileWriter(config)
	case constants.SimpleFileFormat:
		newFileWriter, err = simplewriter.CreateFileWriter(config)
	}

	if err != nil {
		return nil, fmt.Errorf("Error creating the File Writer :%w", err)
	}

	var newFileReader filereader.FileReader

	switch config.StorageFormat {
	case constants.SegmentFileFormat:

		newFileReader, err = segmentreader.CreateNewFileReader(config)
	case constants.SimpleFileFormat:
		newFileReader, err = simplereader.CreateNewFileReader(config)
	}

	if err != nil {
		return nil, fmt.Errorf("Error creating File Reader :%w", err)
	}

	var indexer indexing.Indexer

	switch config.IndexFormat {
	case constants.ValueIndex:
		indexer = &formats.ValueIndexer{
			IndexFormat: constants.ValueIndex,
			Reader:      newFileReader,
		}
	case constants.PositionIndex:
		indexer = &formats.PositionIndexer{
			IndexFormat: constants.PositionIndex,
			Reader:      newFileReader,
		}
	default:
		indexer = &formats.ValueIndexer{
			IndexFormat: constants.ValueIndex,
			Reader:      newFileReader,
		}
	}

	err = indexer.Populate()

	if err != nil {
		newFileWriter.Close()
		newFileReader.Close()
		return nil, fmt.Errorf("Failure to populate indexer")
	}

	var compactor *compaction.Compactor

	if config.IndexFormat == constants.PositionIndex {
		newCompactor, err := compaction.CreateNewCompactor(newFileReader, fileutil.CreateDefaultSegmentConfig())
		if err != nil {
			return nil, fmt.Errorf("Error creating Compactor")
		}

		compactor = newCompactor
	} else {
		compactor = nil
	}

	store := &FileStore{
		filename:    config.FileName,
		fileWriter:  newFileWriter,
		fileReader:  newFileReader,
		indexer:     indexer,
		indexFormat: config.IndexFormat,
		closed:      false,
		version:     config.Version,
		compactor:   compactor,

		stopCompaction: make(chan struct{}),
	}

	if compactor != nil && config.EnableCompaction && config.CompactionInterval > 5*time.Second {
		store.startCompactionLoop(config.CompactionInterval)
	}

	return store, nil
}

func (f *FileStore) Put(K string, V string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return fmt.Errorf("File store is currently closed")
	}

	if K == "" {
		return fmt.Errorf("Key cannot be empty")
	}

	record := types.Record{
		Version:   f.version,
		Operation: constants.PUT_OPERATION,
		Key:       K,
		Value:     V,
	}

	recordLocation, err := f.fileWriter.Append(record)

	fmt.Printf("writing record: %+v\n", record)
	fmt.Printf("writing record location: %+v\n", recordLocation)

	if err != nil {
		return fmt.Errorf("Error appending value to log")
	}

	fmt.Println("record appended successfully")

	var value string

	switch f.indexFormat {
	case constants.PositionIndex:
		value = ""
	case constants.ValueIndex:
		value = V
	default:
		value = V
	}

	indexerEntry := indexing.IndexEntry{
		Value:          value,
		RecordLocation: recordLocation,
	}

	fmt.Println("indexer entry value", indexerEntry)

	f.indexer.Update(K, indexerEntry)

	f.indexer.View()

	fmt.Println("record updated in index successfully")

	return nil
}

func (f *FileStore) Get(K string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if K == "" {
		return "", fmt.Errorf("Key cannot be empty")
	}

	var value string

	entry, exists := f.indexer.Get(K)

	if !exists {
		return "", variables.KeyNotFound
	}

	switch f.indexFormat {
	case constants.PositionIndex:
		record, err := f.fileReader.ReadAtOffset(entry.RecordLocation)

		// fmt.Println("record", record.Key, record.Value)

		if err != nil {
			return "", fmt.Errorf("Error getting key: %w, offset info: %v", err, entry.RecordLocation)
		}

		// it is possible the retrieved key is empty, return delete op
		if record.Operation == constants.DELETE_OPERATION {
			return "", variables.KeyNotFound
		}

		// getting value from file directly
		value = record.Value

	case constants.ValueIndex:
		value = entry.Value
	default:
		value = entry.Value
	}

	return value, nil
}

func (f *FileStore) Delete(K string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if K == "" {
		return fmt.Errorf("Key to be deleted cannot be empty")
	}

	_, exists := f.indexer.Get(K)

	if !exists {
		return fmt.Errorf("Key to be deleted must exist")
	}

	record := types.Record{
		Version:   f.version,
		Operation: constants.DELETE_OPERATION,
		Key:       K,
	}

	if _, err := f.fileWriter.Append(record); err != nil {
		return fmt.Errorf("Error appending delete entry to disk")
	}

	f.indexer.Delete(record.Key)

	return nil
}

func (f *FileStore) CompactSegments() error {
	f.mu.RLock()
	closed := f.closed
	f.mu.RUnlock()

	if closed {
		return fmt.Errorf("Store is already closed")
	}

	if f.compactor == nil {
		fmt.Println("Compactor not enabled.")
		return nil
	}

	compactionResult, err := f.compactor.CompactStaleSegments()

	if err != nil {
		return fmt.Errorf("Error getting compaction results: %w", err)
	}

	// if no data is returned simply return nil
	if len(compactionResult.CompactedSegmentNames) == 0 || len(compactionResult.Index) == 0 {
		fmt.Println("Nothing to compact")
		return nil
	}

	err = f.processCompaction(compactionResult)

	if err != nil {
		return fmt.Errorf("Stale segment compaction failed: %w", err)
	}

	return nil
}

func (f *FileStore) Close() error {

	f.stopOnce.Do(func() {
		close(f.stopCompaction)
	})
	f.compactionWg.Wait()

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return fmt.Errorf("Store is already closed")
	}

	writerErr := f.fileWriter.Close()
	readerErr := f.fileReader.Close()

	f.closed = true

	return errors.Join(writerErr, readerErr)
}

func (f *FileStore) processCompaction(result compaction.CompactorResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// rename old files
	stalePathMapping, err := fileutil.RenameNewSegmentFiles(fileutil.CreateDefaultSegmentConfig(), result.CompactedSegmentNames, result.LastSegmentId)

	if err != nil {
		return fmt.Errorf("Error renaming file segments: %w", err)
	}

	remainingFile := result.StaleFilepaths[0 : len(result.StaleFilepaths)-len(result.CompactedSegmentNames)]

	// delete old files
	err = fileutil.DeleteStaleFileSegments(remainingFile)

	if err != nil {
		return fmt.Errorf("Error deleting file segments: %w", err)
	}

	// update indexer
	for _, value := range result.Index {

		existingEntry, exists := f.indexer.Get(value.Key)

		// if the key does not exist it is likely it has been deleted, so we skip it
		if !exists {
			continue
		}

		// check if the current record is at a higher segment id.
		if exists && existingEntry.RecordLocation.SegmentId > value.RecordLocation.SegmentId {
			continue
		}

		randomSegmentName, exist := result.RecordToCompactedNameIndex[value.Key]

		if !exist {
			return fmt.Errorf("Record does not have a corresponding random segment name, record: %s, random name: %s", value.Key, randomSegmentName)
		}

		newSegmentId, exist := stalePathMapping[randomSegmentName]

		if !exist {
			return fmt.Errorf("Record does not have a corresponding segment ID: random name: %s, segment id: %d", randomSegmentName, newSegmentId)
		}

		newIndexEntry := indexing.IndexEntry{
			Value: "", // compaction is only processed with position based indexing
			RecordLocation: types.RecordLocation{
				SegmentId: newSegmentId,
				Offset:    value.RecordLocation.Offset,
				Size:      value.RecordLocation.Size,
			},
		}

		f.indexer.Update(value.Key, newIndexEntry)
	}

	return nil
}

func (f *FileStore) startCompactionLoop(interval time.Duration) {
	f.compactionWg.Add(1)

	go func() {
		defer f.compactionWg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := f.CompactSegments(); err != nil {
					fmt.Println("Background run failed: %w", err)
				}
			case <-f.stopCompaction:
				fmt.Println("Compaction loop has ended")
				return
			}
		}
	}()
}
