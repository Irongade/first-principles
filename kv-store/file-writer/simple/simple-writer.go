package simplewriter

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
)

type WriterConfig struct {
	FilePath   string
	FileMode   os.FileMode
	BufferSize int
	SyncPolicy types.SyncPolicy
	Formatter  types.FormatterOptions
}

func CreateDefaultWriterConfig(filePath string) WriterConfig {
	return WriterConfig{
		FilePath:   filePath,
		FileMode:   0o600,
		BufferSize: 32 * 1024,
		SyncPolicy: constants.SyncEveryWrite,
		Formatter:  constants.TEXT_FORMATTER,
	}
}

type FileWriter struct {
	file       *os.File
	writer     *bufio.Writer
	formatter  formatter.Formatter
	syncPolicy types.SyncPolicy
	offset     int64
	closed     bool
}

func CreateFileWriter(config WriterConfig) (*FileWriter, error) {
	if config.FilePath == "" {
		return nil, variables.FileNotFound
	}

	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, config.FileMode)
	if err != nil {
		return nil, fmt.Errorf(variables.FileNotOpened.Error()+" :%w", err)
	}

	info, err := file.Stat()

	if err != nil {
		file.Close()
		return nil, fmt.Errorf("Error fetching file stat: %w", err)
	}

	if config.BufferSize <= 0 {
		file.Close()
		return nil, variables.BufferSizeInvalid
	}

	var fmter formatter.Formatter
	switch config.Formatter {
	case constants.TEXT_FORMATTER:
		fmter = &text.TextFormatter{}

	default:
		fmter = &text.TextFormatter{}
	}

	return &FileWriter{
		file:       file,
		writer:     bufio.NewWriterSize(file, config.BufferSize),
		formatter:  fmter,
		syncPolicy: config.SyncPolicy,
		closed:     false,
		offset:     info.Size(),
	}, nil
}

func (f *FileWriter) Append(record types.Record) (types.RecordLocation, error) {
	// if file writer is closed
	if f.closed {
		return types.RecordLocation{}, fmt.Errorf("File writer is closed")
	}

	// encode data into bytes so they can be streamed to file.
	encoded, err := f.formatter.Encode(record)

	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Encode error occurred : %w", err)
	}

	location := types.RecordLocation{
		Offset: f.offset,
		Size:   uint32(len(encoded)),
	}

	// use the buffer to write to memory, and when the buffer size is exceeded it is flushed to
	n, err := f.writer.Write(encoded)

	if err != nil {
		return types.RecordLocation{}, fmt.Errorf("Write record failed : %w", err)
	}

	// perhaps we need to flush and sync every time to make sure data is accurate.
	if n != len(encoded) {
		return types.RecordLocation{}, fmt.Errorf("Written length not equal to encoded data: %w", err)
	}

	// make sure to advance the offset now after write succeeds
	f.offset += int64(n)

	if f.syncPolicy == constants.SyncEveryWrite {
		return location, f.flushAndSync()
	}

	return location, nil
}

func (f *FileWriter) Flush() error {
	if f.closed {
		return fmt.Errorf("File writer is closed")
	}

	return f.flushAndSync()
}

func (f *FileWriter) Close() error {
	if f.closed {
		return fmt.Errorf("File writer is closed")
	}

	f.closed = true

	flushErr := f.writer.Flush()
	syncErr := f.file.Sync()
	closeErr := f.file.Close()

	return errors.Join(flushErr, syncErr, closeErr)
}

func (f *FileWriter) GetActiveSegmentId() (int, error) {
	return 0, fmt.Errorf("Simple file writer does not use active segments")
}

func (f *FileWriter) flushAndSync() error {
	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("Buffer Flush error occurred : %w", err)
	}

	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("File Sync error occurred : %w", err)
	}

	return nil
}
