package storage

import (
	"errors"
	"fmt"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	segmentreader "kvstore/file-reader/segment"
	simplereader "kvstore/file-reader/simple"
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
)

const FILE_PATH_PREFIX = "./data/"

type FileStore struct {
	filename    string
	fileWriter  filewriter.FileWriter
	fileReader  filereader.FileReader
	indexer     indexing.Indexer
	indexFormat indexing.IndexFormat

	mu      sync.RWMutex
	version string
	closed  bool
}

func NewFileStore(filename string, version string, fileFormat types.FileFormat, indexerFormat indexing.IndexFormat) (*FileStore, error) {
	filepath := FILE_PATH_PREFIX + filename

	dir := path.Dir(filepath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	var newFileWriter filewriter.FileWriter
	var err error

	switch fileFormat {
	case constants.SegmentFileFormat:
		segmentFileWriterConfig := segmentwriter.CreateDefaultWriterConfig()
		newFileWriter, err = segmentwriter.CreateFileWriter(segmentFileWriterConfig)
	case constants.SimpleFileFormat:
		simpleFileWriterConfig := simplewriter.CreateDefaultWriterConfig(filepath)
		newFileWriter, err = simplewriter.CreateFileWriter(simpleFileWriterConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("Error creating the File Writer :%w", err)
	}

	var newFileReader filereader.FileReader

	switch fileFormat {
	case constants.SegmentFileFormat:
		segmentFileReaderConfig := segmentreader.CreateDefaultReaderConfig()
		newFileReader, err = segmentreader.CreateNewFileReader(segmentFileReaderConfig)
	case constants.SimpleFileFormat:
		simpleFileReaderConfig := simplereader.CreateDefaultReaderConfig(filepath)
		newFileReader, err = simplereader.CreateNewFileReader(simpleFileReaderConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("Error creating File Reader :%w", err)
	}

	var indexer indexing.Indexer
	switch indexerFormat {
	case indexing.BasicIndex:
		indexer = &formats.BasicIndexer{
			IndexFormat: indexing.BasicIndex,
			Reader:      newFileReader,
		}
	case indexing.PositionIndex:
		indexer = &formats.PositionIndexer{
			IndexFormat: indexing.PositionIndex,
			Reader:      newFileReader,
		}
	default:
		indexer = &formats.BasicIndexer{
			IndexFormat: indexing.BasicIndex,
			Reader:      newFileReader,
		}
	}

	err = indexer.Populate()

	if err != nil {
		newFileWriter.Close()
		newFileReader.Close()
		return nil, fmt.Errorf("Failure to populate indexer")
	}

	return &FileStore{
		filename:    filename,
		fileWriter:  newFileWriter,
		fileReader:  newFileReader,
		indexer:     indexer,
		indexFormat: indexerFormat,
		closed:      false,
		version:     version,
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

	recordLocation, err := f.fileWriter.Append(record)

	fmt.Printf("writing record: %+v\n", record)
	fmt.Printf("writing record location: %+v\n", recordLocation)

	if err != nil {
		return fmt.Errorf("Error appending value to log")
	}

	fmt.Println("record appended successfully")

	var value string

	switch f.indexFormat {
	case indexing.PositionIndex:
		value = ""
	case indexing.BasicIndex:
		value = ""
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
	f.mu.Lock()
	defer f.mu.Unlock()

	if K == "" {
		return "", fmt.Errorf("Key cannot be empty")
	}

	var value string

	entry, exists := f.indexer.Get(K)

	if !exists {
		return "", variables.KeyNotFound
	}

	switch f.indexFormat {
	case indexing.PositionIndex:
		record, err := f.fileReader.ReadAtOffset(entry.RecordLocation)

		fmt.Println("record", record.Key, record.Value)

		if err != nil {
			return "", fmt.Errorf("Error getting key: %w, offset info: %v", err, entry.RecordLocation)
		}

		// it is possible the retrieved key is empty, return delete op
		if record.Operation == constants.DELETE_OPERATION {
			return "", variables.KeyNotFound
		}

		// getting value from file directly
		value = record.Value

	case indexing.BasicIndex:
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
