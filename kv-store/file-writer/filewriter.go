package filewriter

import (
	"bufio"
	"errors"
	"fmt"
	"kvstore/constants"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"os"
)

type WriterConfig struct {
	FilePath   string
	FileMode   os.FileMode
	BufferSize int
	SyncPolicy types.SyncPolicy
	Formatter  int
}

func CreateDefaultWriterConfig(filePath string) WriterConfig {
	return WriterConfig{
		FilePath:   filePath,
		FileMode:   0o600,
		BufferSize: 32 * 1024,
		SyncPolicy: constants.SyncEveryWrite,
		Formatter:  formatter.TEXT_FORMATTER,
	}
}

type FileWriter struct {
	file       *os.File
	writer     *bufio.Writer
	formatter  formatter.Formatter
	syncPolicy types.SyncPolicy
	closed     bool
}

func CreateFileWriter(config WriterConfig) (*FileWriter, error) {
	if config.FilePath == "" {
		return nil, FileNotFound
	}

	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, config.FileMode)
	if err != nil {
		return nil, fmt.Errorf(FileNotOpened.Error()+" :%w", err)
	}

	if config.BufferSize <= 0 {
		file.Close()
		return nil, BufferSizeInvalid
	}

	var fmter formatter.Formatter
	switch config.Formatter {
	case formatter.TEXT_FORMATTER:
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
	}, nil
}

func (f *FileWriter) Append(record types.Record) error {
	// if file writer is closed
	if f.closed {
		return fmt.Errorf("File writer is closed")
	}

	// encode data into bytes so they can be streamed to file.
	encoded, err := f.formatter.Encode(record)

	if err != nil {
		return fmt.Errorf("Encode error occurred : %w", err)
	}

	// use the buffer to write to memory, and when the buffer size is exceeded it is flushed to
	if _, err := f.writer.Write(encoded); err != nil {
		return fmt.Errorf("Write record failed : %w", err)
	}

	if f.syncPolicy == constants.SyncEveryWrite {
		return f.flushAndSync()
	}

	return nil
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

func (f *FileWriter) flushAndSync() error {
	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("Buffer Flush error occurred : %w", err)
	}

	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("File Sync error occurred : %w", err)
	}

	return nil
}
