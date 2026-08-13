package segmentwriter

import (
	"errors"
	"fmt"
	"io/fs"
	"kvstore/config"
	"kvstore/constants"
	fileutil "kvstore/file-util"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"os"
)

type WriterConfig struct {
	FileDirectory       string
	BufferSize          int
	SyncPolicy          types.SyncPolicy
	Formatter           types.FormatterOptions
	FilePermission      fs.FileMode
	DirectoryPermission fs.FileMode
	MaxSegmentSize      int64
}

type FileWriter struct {
	current   *fileutil.Segment
	threshold int64
	formatter formatter.Formatter
	config    config.Config
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

func CreateFileWriter(config config.Config) (*FileWriter, error) {
	if config.DirectoryPermission == 0 {
		return nil, fmt.Errorf("Directory Permissions cannot be invalid")
	}

	if config.FilePermission == 0 {
		return nil, fmt.Errorf("File Permissions cannot be invalid")
	}

	if err := os.MkdirAll(config.DataDir, os.FileMode(config.DirectoryPermission)); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	index, err := fileutil.FindLastSegment(fileutil.SegmentConfig{
		FileDirectory: config.DataDir,
	})

	if err != nil {
		return nil, fmt.Errorf("Error reading last segment id: %w", err)
	}

	segment, err := fileutil.OpenFileSegment(fileutil.SegmentConfig{
		FileDirectory:  config.DataDir,
		BufferSize:     config.BufferSize,
		FilePermission: config.FilePermission,
	}, index)

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
		threshold: int64(config.MaxSegmentSize),
	}, nil
}

func (f *FileWriter) Append(record types.Record) (types.RecordLocation, error) {
	if f.current.Closed {
		return types.RecordLocation{}, fmt.Errorf("Segment File writer is closed")
	}

	encoded, err := f.formatter.Encode(record)
	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Error encoding record: %w", err)
	}
	encodedSize := len(encoded)

	// if the threshold is surpassed roll the segments.
	if f.current.Offset > 0 && f.current.Offset+int64(encodedSize) > f.threshold {
		if err := f.roll(); err != nil {
			return types.RecordLocation{}, err
		}
	}

	location := types.RecordLocation{
		SegmentId: f.current.Id,
		Offset:    f.current.Offset,
		Size:      uint32(encodedSize),
	}

	// fmt.Println("location value", location)
	// n, err := f.current.file.Write(encoded)
	// this could be done, but for now, keeping it simple
	n, err := f.current.Writer.Write(encoded)

	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Error appending record: %w", err)
	}

	if n <= 0 || n != encodedSize {
		return types.RecordLocation{}, fmt.Errorf("Error appending a record with invalid size: %d", n)
	}

	// advance offset
	f.current.Offset += int64(n)

	if f.config.SyncPolicy == constants.SyncEveryWrite {
		return location, f.Flush()
	}

	return location, nil

}

func (f *FileWriter) Flush() error {
	if f.current.Closed {
		return nil
	}

	if err := f.current.Writer.Flush(); err != nil {
		return err
	}

	return f.current.File.Sync()
}

func (f *FileWriter) Close() error {
	if f.current.Closed {
		return nil
	}

	flushErr := f.Flush()
	closeErr := f.current.File.Close()

	// close the current segment.
	f.current.Closed = true

	return errors.Join(flushErr, closeErr)
}

func (f *FileWriter) GetActiveSegmentId() (int, error) {
	if f.current.Closed {
		return 0, fmt.Errorf("File writer has been closed, cannot retrieve active segment id")
	}

	return f.current.Id, nil
}

func (f *FileWriter) roll() error {
	if f.current.Closed {
		return fmt.Errorf("Cannot roll segment as file writer is closed")
	}

	// flush the contents to disk
	if err := f.Flush(); err != nil {
		return fmt.Errorf("Error flushing file during rolling segment update: %w", err)
	}

	// after flushing close the file so we dont have files stuck in memory
	if err := f.current.File.Close(); err != nil {
		return fmt.Errorf("Error closing file during rolling segment update: %w", err)
	}

	// close off the file so other ops can fail.
	f.current.Closed = true

	prevIndex := f.current.Id

	newSegment, err := fileutil.OpenFileSegment(fileutil.SegmentConfig{
		FileDirectory:  f.config.DataDir,
		BufferSize:     f.config.BufferSize,
		FilePermission: f.config.FilePermission,
	}, prevIndex+1)

	if err != nil {
		return fmt.Errorf("Error opening new segment file: %w", err)
	}

	// update new segment and its id
	f.current = newSegment

	return nil
}
