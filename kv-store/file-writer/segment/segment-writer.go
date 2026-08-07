package segmentwriter

import (
	"bufio"
	"errors"
	"fmt"
	"kvstore/constants"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"kvstore/variables"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WriterConfig struct {
	FileDirectory       string
	BufferSize          int
	SyncPolicy          types.SyncPolicy
	Formatter           types.FormatterOptions
	FilePermission      int
	DirectoryPermission int
	MaxSegmentSize      int64
}

type Segment struct {
	id     int
	file   *os.File
	writer *bufio.Writer
	offset int64
	closed bool
}

type FileWriter struct {
	current   *Segment
	threshold int64
	formatter formatter.Formatter
	config    WriterConfig
}

func CreateDefaultWriterConfig() WriterConfig {
	return WriterConfig{
		FileDirectory:       "data",
		BufferSize:          256,
		SyncPolicy:          constants.SyncEveryWrite,
		Formatter:           constants.TEXT_FORMATTER,
		MaxSegmentSize:      2 * 1024,
		FilePermission:      constants.FILE_PERMISSION,
		DirectoryPermission: constants.DIRECTORY_PERMISSION,
	}
}

func CreateFileWriter(config WriterConfig) (*FileWriter, error) {
	if config.DirectoryPermission == 0 {
		return nil, fmt.Errorf("Directory Permissions cannot be invalid")
	}

	if config.FilePermission == 0 {
		return nil, fmt.Errorf("File Permissions cannot be invalid")
	}

	if err := os.MkdirAll(config.FileDirectory, os.FileMode(config.DirectoryPermission)); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	index, err := findLastSegment(config)

	if err != nil {
		return nil, fmt.Errorf("Error reading last segment id: %w", err)
	}

	segment, err := openSegment(config, index)

	if err != nil {
		return nil, fmt.Errorf("Error opening segment file with id %d: %w", index, err)
	}

	var fmter formatter.Formatter
	switch config.Formatter {
	case constants.TEXT_FORMATTER:
		fmter = &text.TextFormatter{}

	default:
		fmter = &text.TextFormatter{}
	}

	return &FileWriter{
		current:   segment,
		formatter: fmter,
		config:    config,
		threshold: config.MaxSegmentSize,
	}, nil
}

func (f *FileWriter) Append(record types.Record) (types.RecordLocation, error) {
	if f.current.closed {
		return types.RecordLocation{}, fmt.Errorf("Segment File writer is closed")
	}

	encoded, err := f.formatter.Encode(record)
	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Error encoding record: %w", err)
	}
	encodedSize := len(encoded)

	// if the threshold is surpassed roll the segments.
	if f.current.offset > 0 && f.current.offset+int64(encodedSize) > f.threshold {
		if err := f.roll(); err != nil {
			return types.RecordLocation{}, err
		}
	}

	location := types.RecordLocation{
		SegmentId: f.current.id,
		Offset:    f.current.offset,
		Size:      uint32(encodedSize),
	}

	fmt.Println("location value", location)
	// n, err := f.current.file.Write(encoded)
	// this could be done, but for now, keeping it simple
	n, err := f.current.writer.Write(encoded)

	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Error appending record: %w", err)
	}

	if n <= 0 || n != encodedSize {
		return types.RecordLocation{}, fmt.Errorf("Error appending a record with invalid size: %d", n)
	}

	// advance offset
	f.current.offset += int64(n)

	if f.config.SyncPolicy == constants.SyncEveryWrite {
		return location, f.Flush()
	}

	return location, nil

}

func (f *FileWriter) Flush() error {
	if f.current.closed {
		return nil
	}

	if err := f.current.writer.Flush(); err != nil {
		return err
	}

	return f.current.file.Sync()
}

func (f *FileWriter) Close() error {
	if f.current.closed {
		return nil
	}

	flushErr := f.Flush()
	closeErr := f.current.file.Close()

	// close the current segment.
	f.current.closed = true

	return errors.Join(flushErr, closeErr)
}

func (f *FileWriter) roll() error {
	if f.current.closed {
		return fmt.Errorf("Cannot roll segment as file writer is closed")
	}

	// flush the contents to disk
	if err := f.Flush(); err != nil {
		return fmt.Errorf("Error flushing file during rolling segment update: %w", err)
	}

	// after flushing close the file so we dont have files stuck in memory
	if err := f.current.file.Close(); err != nil {
		return fmt.Errorf("Error closing file during rolling segment update: %w", err)
	}

	// close off the file so other ops can fail.
	f.current.closed = true

	prevIndex := f.current.id

	newSegment, err := openSegment(f.config, prevIndex+1)

	if err != nil {
		return fmt.Errorf("Error opening new segment file: %w", err)
	}

	// update new segment and its id
	f.current = newSegment

	return nil
}

func findLastSegment(config WriterConfig) (int, error) {

	if config.FilePermission == 0 {
		return 0, fmt.Errorf("File Permissions cannot be invalid")
	}

	entries, err := os.ReadDir(config.FileDirectory)

	if err != nil {
		return 0, fmt.Errorf("Error opening Segment directory")
	}

	last := 1

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, constants.SEGMENT_PREFIX) || !strings.HasSuffix(name, constants.SEGMENT_EXTENSION) {
			continue
		}

		trimmedName := strings.TrimSuffix(strings.TrimPrefix(name, constants.SEGMENT_PREFIX), constants.SEGMENT_EXTENSION)

		id, err := strconv.Atoi(trimmedName)

		if err != nil {
			continue
		}

		if id > last {
			last = id
		}

	}

	return last, nil
}

func openSegment(config WriterConfig, index int) (*Segment, error) {

	filepath := filepath.Join(config.FileDirectory, getSegmentName(index))

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.FileMode(config.FilePermission))

	if err != nil {
		return nil, fmt.Errorf(variables.FileNotOpened.Error()+" :%w", err)
	}

	info, err := file.Stat()

	if err != nil {
		file.Close()
		return nil, fmt.Errorf("File Stats could not be fetched")
	}

	if config.BufferSize <= 0 {
		file.Close()
		return nil, variables.BufferSizeInvalid
	}

	fmt.Println("Open segment", index, file.Name())

	return &Segment{
		id:     index,
		file:   file,
		writer: bufio.NewWriterSize(file, config.BufferSize),
		offset: info.Size(),
		closed: false,
	}, nil
}

func getSegmentName(id int) string {
	return fmt.Sprintf("%s%0*d%s", constants.SEGMENT_PREFIX, constants.SEGMENT_NAME_WIDTH, id, constants.SEGMENT_EXTENSION)
}
