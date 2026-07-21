package storage

import (
	"errors"
	"fmt"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	filewriter "kvstore/file-writer"
	"kvstore/indexing"
	"kvstore/indexing/formats"
	"kvstore/types"
	"sync"
)

const FILE_PATH_PREFIX = "./data"

type FileStore struct {
	filename   string
	fileWriter *filewriter.FileWriter
	fileReader *filereader.FileReader
	indexer    indexing.Indexer

	mu      sync.RWMutex
	version string
	closed  bool
}

func NewFileStore(filename string, version string, indexerFormat indexing.IndexFormat) (*FileStore, error) {
	filepath := FILE_PATH_PREFIX + filename

	fileWriterConfig := filewriter.CreateDefaultWriterConfig(filepath)
	fileReaderConfig := filereader.CreateDefaultReaderConfig(filepath)

	newFileWriter, err := filewriter.CreateFileWriter(fileWriterConfig)

	if err != nil {
		return nil, fmt.Errorf("Error creating the File Writer :%w", err)
	}

	newFileReader, err := filereader.CreateNewFileReader(fileReaderConfig)

	if err != nil {
		return nil, fmt.Errorf("Error creating File Reader :%w", err)
	}

	fmt.Println(newFileReader, newFileWriter)

	// load up all the logs in memory in our index, for fast gets and updates.
	records, err := newFileReader.ReadAll()

	if err != nil {
		newFileWriter.Close()
		newFileReader.Close()
		return nil, fmt.Errorf("Error reading current log state: %w", err)
	}

	var indexer indexing.Indexer
	switch indexerFormat {
	default:
		indexer = &formats.BasicIndexer{
			IndexFormat: indexing.BasicIndex,
		}
	}

	err = indexer.Populate(records)

	if err != nil {
		return nil, fmt.Errorf("Failure to populate indexer")
	}

	return &FileStore{
		filename:   filename,
		fileWriter: newFileWriter,
		fileReader: newFileReader,
		indexer:    indexer,
		closed:     false,
		version:    version,
	}, nil
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

	err := f.fileWriter.Append(record)

	if err != nil {
		return fmt.Errorf("Error appending value to log")
	}

	indexerEntry := indexing.IndexEntry{
		Value: V,
	}

	f.indexer.Update(K, indexerEntry)

	return nil
}

func (f *FileStore) Get(K string) (string, error) {
	f.mu.RLock()
	defer f.mu.Unlock()

	if K == "" {
		return "", fmt.Errorf("Key cannot be empty")
	}

	var value string

	entry, exists := f.indexer.Get(K)

	if !exists {
		return "", fmt.Errorf("Key does not exist in the store")
	}

	value = entry.Value

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
		return fmt.Errorf("Key to be deleted must exists")
	}

	record := types.Record{
		Version:   f.version,
		Operation: constants.DELETE_OPERATION,
		Key:       K,
	}

	f.fileWriter.Append(record)

	f.indexer.Delete(record.Key)

	return nil
}

func (f *FileStore) Close() error {
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
