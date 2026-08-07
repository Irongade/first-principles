package segmentreader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"kvstore/constants"
	fileutil "kvstore/file-util"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"os"
	"path/filepath"
)

type Segment struct {
	id     int
	file   *os.File
	closed bool
}

type FileReader struct {
	formatter formatter.Formatter
	config    ReaderConfig
}

type ReaderConfig struct {
	FileDirectory       string
	Formatter           types.FormatterOptions
	MaxRecordSize       int
	BufferSize          int
	FilePermission      int
	DirectoryPermission int
	MaxSegmentSize      int64
}

func CreateDefaultReaderConfig() ReaderConfig {
	return ReaderConfig{
		FileDirectory:       "data",
		BufferSize:          256,
		Formatter:           constants.TEXT_FORMATTER,
		MaxSegmentSize:      2 * 1024,
		FilePermission:      constants.FILE_PERMISSION,
		DirectoryPermission: constants.DIRECTORY_PERMISSION,
		MaxRecordSize:       4 * 1024 * 1024,
	}
}

func CreateNewFileReader(config ReaderConfig) (*FileReader, error) {

	if config.MaxRecordSize <= 0 {
		return nil, fmt.Errorf("Max record size cannot be less than or equal to 0")
	}

	var fmter formatter.Formatter
	switch config.Formatter {
	case constants.TEXT_FORMATTER:
		fmter = &text.TextFormatter{}

	default:
		fmter = &text.TextFormatter{}
	}

	return &FileReader{
		formatter: fmter,
		config:    config,
	}, nil
}

func (r *FileReader) ReadAtOffset(location types.RecordLocation) (types.Record, error) {
	if location.SegmentId == 0 {
		return types.Record{}, fmt.Errorf("Record must have a segment id")
	}

	if location.Size == 0 {
		return types.Record{}, fmt.Errorf("Fetched data size cannot be invalid")
	}

	filepath := filepath.Join(r.config.FileDirectory, fileutil.GetSegmentName(location.SegmentId))

	segmentFile, err := os.OpenFile(filepath, os.O_RDONLY, 0)

	if err != nil {
		return types.Record{}, fmt.Errorf("Segement Id does not exist for this read operation")
	}

	buffer := make([]byte, location.Size)

	n, err := segmentFile.ReadAt(buffer, location.Offset)

	if err != nil && !errors.Is(err, io.EOF) {
		return types.Record{}, fmt.Errorf("Seeking at offset failed: %w", err)
	}

	if n != len(buffer) {
		return types.Record{}, fmt.Errorf("Fetched data mismatch: %w", err)
	}

	record, err := r.formatter.Decode(buffer)

	if err != nil {
		return types.Record{}, fmt.Errorf("Error decoding bytes: %w", err)
	}

	err = segmentFile.Close()

	if err != nil {
		return types.Record{}, fmt.Errorf("Error closing file after scanning: %w", err)
	}

	return record, nil
}

func (r *FileReader) ReadAll() ([]types.Record, error) {
	filepaths, err := fileutil.GetAllSegmentFilePaths(fileutil.SegmentConfig{
		FileDirectory: r.config.FileDirectory,
	})

	if err != nil {
		return []types.Record{}, fmt.Errorf("Error reading all records due to segment file paths")
	}

	records := make([]types.Record, 0)

	for _, filepath := range filepaths {
		fileRecords, err := r.readFile(filepath)

		if err != nil {
			return nil, fmt.Errorf("Error reading this file: %s, err details here: %w", filepath, err)
		}

		records = append(records, fileRecords...)
	}

	return records, nil
}

// Specifically for tracking offsets
func (r *FileReader) ScanAll() ([]types.ScannedRecord, error) {
	filepaths, err := fileutil.GetAllSegmentFilePaths(fileutil.SegmentConfig{
		FileDirectory: r.config.FileDirectory,
	})

	if err != nil {
		return nil, fmt.Errorf("Error reading all records due to segment file paths")
	}

	records := make([]types.ScannedRecord, 0)

	for _, filepath := range filepaths {
		fileRecords, err := r.scanFile(filepath)

		if err != nil {
			return nil, fmt.Errorf("Error reading this file: %s, err details here: %w", filepath, err)
		}

		records = append(records, fileRecords...)
	}

	return records, nil
}

func (r *FileReader) readFile(filepath string) ([]types.Record, error) {
	file, err := os.OpenFile(filepath, os.O_RDONLY, 0)

	if err != nil {
		return nil, fmt.Errorf("Error opening file: %s", filepath)
	}

	_, err = file.Seek(0, io.SeekStart)

	if err != nil {
		return nil, fmt.Errorf("Seek to beginning failed: %w", err)
	}

	scanner := newScanner(file, r.config.MaxRecordSize)

	records := make([]types.Record, 0)

	for scanner.Scan() {
		line := scanner.Bytes()

		record, err := r.formatter.Decode(line)

		if err != nil {
			return nil, fmt.Errorf("Error decoding bytes: %w", err)
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Scanning file error: %w", err)
	}

	err = file.Close()

	if err != nil {
		return nil, fmt.Errorf("Error closing file after scanning: %w", err)
	}

	return records, nil
}

func (r *FileReader) scanFile(filepath string) ([]types.ScannedRecord, error) {
	file, err := os.OpenFile(filepath, os.O_RDONLY, 0)

	if err != nil {
		return nil, fmt.Errorf("Error opening file: %s", filepath)
	}

	_, err = file.Seek(0, io.SeekStart)

	if err != nil {
		return nil, fmt.Errorf("Seek to beginning failed: %w", err)
	}

	segmentId, err := fileutil.GetSegmentId(filepath)

	if err != nil {
		return nil, fmt.Errorf("Error getting segment id from scanFile: %s, error is: %w", filepath, err)
	}

	var offset int64

	scanner := newScanner(file, r.config.MaxRecordSize)

	records := make([]types.ScannedRecord, 0)

	for scanner.Scan() {
		line := scanner.Bytes()

		record, err := r.formatter.Decode(line)

		if err != nil {
			return nil, fmt.Errorf("Error decoding bytes: %w", err)
		}

		// adding + 1 to include new line since .Scan() removes them.
		size := len(line) + 1

		records = append(records, types.ScannedRecord{
			Record: record,
			RecordLocation: types.RecordLocation{
				SegmentId: segmentId,
				Offset:    offset,
				Size:      uint32(size),
			},
		})

		// advance offset forward.
		offset += int64(size)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Scanning file error: %w", err)
	}

	err = file.Close()

	if err != nil {
		return nil, fmt.Errorf("Error closing file after scanning: %w", err)
	}

	return records, nil
}

func (r *FileReader) Close() error {
	return nil
}

func newScanner(file *os.File, maxRecordSize int) *bufio.Scanner {
	scanner := bufio.NewScanner(file)

	initialBuffer := make([]byte, 64*1024)

	scanner.Buffer(initialBuffer, maxRecordSize)

	return scanner
}
