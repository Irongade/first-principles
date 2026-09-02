package simple

import (
	"errors"
	"fmt"
	"os"
	"statemachine/types"
)

type WriterConfig struct {
	FilePath       string
	FilePermission os.FileMode
}

func CreateDefaultWriterConfig(filePath string) WriterConfig {
	return WriterConfig{
		FilePath:       filePath,
		FilePermission: 0o644,
	}
}

type FileWriter struct {
	file   *os.File
	offset int64
	closed bool
}

func CreateFileWriter(filePath string, filePermission os.FileMode) (*FileWriter, error) {
	if filePath == "" {
		return nil, fmt.Errorf("File name cannot be empty")
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermission)
	if err != nil {
		return nil, fmt.Errorf("Error opening file"+" :%w", err)
	}

	info, err := file.Stat()

	if err != nil {
		file.Close()
		return nil, fmt.Errorf("Error fetching file stat: %w", err)
	}

	return &FileWriter{
		file:   file,
		closed: false,
		offset: info.Size(),
	}, nil
}

func (f *FileWriter) Append(data []byte) (types.Location, error) {
	// if file writer is closed
	if f.closed {
		return types.Location{}, fmt.Errorf("File writer is closed")
	}

	location := types.Location{
		Offset: f.offset,
		Size:   uint32(len(data)),
	}

	// use the buffer to write to memory, and when the buffer size is exceeded it is flushed to
	n, err := f.file.Write(data)

	if err != nil {
		return types.Location{}, fmt.Errorf("Write record failed : %w", err)
	}

	// perhaps we need to flush and sync every time to make sure data is accurate.
	if n != len(data) {
		return types.Location{}, fmt.Errorf("Written length not equal to encoded data: %w", err)
	}

	// make sure to advance the offset now after write succeeds
	f.offset += int64(n)

	return location, nil
}

func (f *FileWriter) Close() error {
	if f.closed {
		return fmt.Errorf("File writer is closed")
	}

	f.closed = true

	syncErr := f.file.Sync()
	closeErr := f.file.Close()

	return errors.Join(syncErr, closeErr)
}
