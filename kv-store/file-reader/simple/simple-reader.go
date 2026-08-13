package simplereader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"os"
)

type FileReader struct {
	file          *os.File
	formatter     formatter.Formatter
	maxRecordSize int
}

type ReaderConfig struct {
	FilePath      string
	Formatter     types.FormatterOptions
	MaxRecordSize int
}

func CreateDefaultReaderConfig(filepath string) ReaderConfig {
	return ReaderConfig{
		FilePath:      filepath,
		Formatter:     constants.TEXT_FORMATTER,
		MaxRecordSize: 4 * 1024 * 1024,
	}
}

func CreateNewFileReader(config ReaderConfig) (*FileReader, error) {
	if config.FilePath == "" {
		return nil, filereader.FileNotFound
	}

	file, err := os.OpenFile(config.FilePath, os.O_RDONLY, 0)

	if err != nil {
		return nil, err
	}

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
		file:          file,
		formatter:     fmter,
		maxRecordSize: config.MaxRecordSize,
	}, nil
}

func (r *FileReader) ReadAtOffset(location types.RecordLocation) (types.Record, error) {
	if location.Size == 0 {
		return types.Record{}, fmt.Errorf("Fetched data size cannot be invalid")
	}

	buffer := make([]byte, location.Size)

	n, err := r.file.ReadAt(buffer, location.Offset)

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

	return record, nil
}

func (r *FileReader) ReadAll() ([]types.Record, error) {
	_, err := r.file.Seek(0, io.SeekStart)

	if err != nil {
		return nil, fmt.Errorf("Seek to beginning failed: %w", err)
	}

	scanner := r.newScanner()

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

	return records, nil
}

// Specifically for tracking offsets
func (r *FileReader) ScanAll() ([]types.ScannedRecord, error) {
	_, err := r.file.Seek(0, io.SeekStart)

	if err != nil {
		return nil, fmt.Errorf("Seek to beginning failed: %w", err)
	}

	var offset int64

	scanner := r.newScanner()

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
				Offset: offset,
				Size:   uint32(size),
			},
		})

		// advance offset forward.
		offset += int64(size)
	}

	return records, nil
}

func (r *FileReader) ScanStaleSegments() ([]types.ScannedRecord, []string, error) {
	return nil, nil, nil
}

func (r *FileReader) Close() error {
	return r.file.Close()
}

func (r *FileReader) newScanner() *bufio.Scanner {
	scanner := bufio.NewScanner(r.file)

	initialBuffer := make([]byte, 64*1024)

	scanner.Buffer(initialBuffer, r.maxRecordSize)

	return scanner
}
