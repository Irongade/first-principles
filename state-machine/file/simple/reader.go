package simple

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"statemachine/constants"
	"statemachine/encoding"
	"statemachine/types"
)

type FileReader struct {
	file *os.File
}

type ReaderConfig struct {
	FilePath string
}

func CreateDefaultReaderConfig(filepath string) ReaderConfig {
	return ReaderConfig{
		FilePath: filepath,
	}
}

func CreateFileReader(filePath string) (*FileReader, error) {
	if filePath == "" {
		return nil, fmt.Errorf("File Name cannot be empty")
	}

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)

	if err != nil {
		return nil, err
	}

	return &FileReader{
		file: file,
	}, nil
}

func (r *FileReader) Read(location types.Location) ([]byte, error) {
	if location.Size == 0 {
		return nil, fmt.Errorf("Fetched data size cannot be invalid")
	}

	buffer := make([]byte, location.Size)

	n, err := r.file.ReadAt(buffer, location.Offset)

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("Seeking at offset failed: %w", err)
	}

	if n != len(buffer) {
		return nil, fmt.Errorf("Fetched data mismatch: %w", err)
	}

	return buffer, nil
}

func (r *FileReader) RebuildIndex() (map[uint64]types.Location, uint64, uint64, error) {
	var offset int64
	var lastIndex uint64
	var firstIndex uint64

	entries := make(map[uint64]types.Location)

	for {
		header := make([]byte, constants.EntryHeaderSize)

		n, err := r.file.ReadAt(header, offset)

		if errors.Is(err, io.EOF) && n == 0 {
			break
		}

		if err != nil && !errors.Is(err, io.EOF) {
			return nil, 0, 0, err
		}

		if n != constants.EntryHeaderSize {
			break
		}

		dataLength := binary.BigEndian.Uint32(header[20:24])

		entrySize := int(dataLength) + constants.EntryHeaderSize

		buf := make([]byte, entrySize)

		bufLen, err := r.file.ReadAt(buf, offset)

		if err != nil && !errors.Is(err, io.EOF) {
			return nil, 0, 0, err
		}

		if bufLen != entrySize {
			// partial entry
			break
		}

		entry, err := encoding.DecodeEntry(buf)

		if err != nil {
			return nil, 0, 0, fmt.Errorf("Error decoding entry: %w", err)
		}

		entries[entry.Index] = types.Location{
			Offset: offset,
			Size:   uint32(entrySize),
		}

		if firstIndex == 0 {
			firstIndex = entry.Index
		}

		if lastIndex < uint64(entry.Index) {
			lastIndex = entry.Index
		}

		offset += int64(entrySize)
	}

	return entries, firstIndex, lastIndex, nil
}
