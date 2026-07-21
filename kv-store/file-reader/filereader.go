package filereader

import (
	"bufio"
	"fmt"
	"io"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"os"
)

type ReaderConfig struct {
	FilePath      string
	Formatter     int
	MaxRecordSize int
}

func CreateDefaultReaderConfig(filepath string) ReaderConfig {
	return ReaderConfig{
		FilePath:      filepath,
		Formatter:     formatter.TEXT_FORMATTER,
		MaxRecordSize: 4 * 1024 * 1024,
	}
}

type FileReader struct {
	file          *os.File
	formatter     formatter.Formatter
	maxRecordSize int
}

func CreateNewFileReader(config ReaderConfig) (*FileReader, error) {
	if config.FilePath == "" {
		return nil, FileNotFound
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
	case formatter.TEXT_FORMATTER:
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

func (r *FileReader) Close() error {
	return r.file.Close()
}

func (r *FileReader) newScanner() *bufio.Scanner {
	scanner := bufio.NewScanner(r.file)

	initialBuffer := make([]byte, 64*1024)

	scanner.Buffer(initialBuffer, r.maxRecordSize)

	return scanner
}
