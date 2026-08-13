package fileutil

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"kvstore/constants"
	"kvstore/types"
	"kvstore/variables"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type Segment struct {
	Id     int
	File   *os.File
	Writer *bufio.Writer
	Offset int64
	Closed bool
}

type SegmentConfig struct {
	FileDirectory           string
	BufferSize              int
	FilePermission          int
	MaxSegmentSize          int
	StaleSegmentFileMaxSize int
	FormatOptions           types.FormatterOptions
}

func CreateDefaultSegmentConfig() SegmentConfig {
	return SegmentConfig{
		FileDirectory:           "data",
		BufferSize:              256,
		FormatOptions:           constants.TEXT_FORMATTER,
		MaxSegmentSize:          2 * 1024,
		FilePermission:          constants.FILE_PERMISSION,
		StaleSegmentFileMaxSize: 1,
	}
}

func FindLastSegment(config SegmentConfig) (int, error) {
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

		id, err := GetSegmentId(name)

		if err != nil {
			continue
		}

		if id > last {
			last = id
		}

	}

	return last, nil
}

func GetAllSegmentFilePaths(config SegmentConfig) ([]string, error) {
	entries, err := os.ReadDir(config.FileDirectory)

	if err != nil {
		return []string{}, fmt.Errorf("Error opening Segment directory")
	}

	validFiles := make([]string, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasPrefix(entry.Name(), constants.SEGMENT_PREFIX) || !strings.HasSuffix(entry.Name(), constants.SEGMENT_EXTENSION) {
			continue
		}

		filepath := filepath.Join(config.FileDirectory, entry.Name())

		validFiles = append(validFiles, filepath)
	}

	// fmt.Println("file paths", validFiles)

	return validFiles, nil
}

func GetSegmentName(id int) string {
	return fmt.Sprintf("%s%0*d%s", constants.SEGMENT_PREFIX, constants.SEGMENT_NAME_WIDTH, id, constants.SEGMENT_EXTENSION)
}

func GetSegmentId(path string) (int, error) {
	name := filepath.Base(path)

	trimmedName := strings.TrimSuffix(strings.TrimPrefix(name, constants.SEGMENT_PREFIX), constants.SEGMENT_EXTENSION)

	id, err := strconv.Atoi(trimmedName)

	if err != nil {
		return 0, fmt.Errorf("Error retrieving segment id value")
	}

	return id, nil
}

func OpenFileSegment(config SegmentConfig, index int) (*Segment, error) {

	filepath := filepath.Join(config.FileDirectory, GetSegmentName(index))

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

	// fmt.Println("Open segment", index, file.Name())

	return &Segment{
		Id:     index,
		File:   file,
		Writer: bufio.NewWriterSize(file, config.BufferSize),
		Offset: info.Size(),
		Closed: false,
	}, nil
}

func GetAllStaleSegmentFilePaths(config SegmentConfig) ([]string, error) {
	if config.FileDirectory == "" {
		return nil, fmt.Errorf("File directory is mandatory")
	}

	segmentFiles, err := GetAllSegmentFilePaths(config)

	if err != nil {
		return nil, fmt.Errorf("Error getting all segment files for use")
	}

	length := len(segmentFiles)

	if length < 2 {
		return []string{}, nil
	}

	// we are only keeping the active segment and prev segment from it, others are stale.
	staleSegmentLength := length - 2

	if staleSegmentLength < config.StaleSegmentFileMaxSize {
		return []string{}, nil
	}

	return segmentFiles[0 : length-2], nil
}

func GetRandomWord(length int) (string, error) {
	word := make([]byte, length)

	for i := range word {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(constants.LETTERS))))

		if err != nil {
			return "", err
		}

		word[i] = constants.LETTERS[n.Int64()]
	}

	return string(word), nil
}

func GetRandomSegmentName(length int) (string, error) {
	word, err := GetRandomWord(length)

	if err != nil {
		return "", fmt.Errorf("Error generating random segment name")
	}

	if word == "" {
		return "", fmt.Errorf("Generated word cannot be empty")
	}

	return strings.Join([]string{word, constants.SEGMENT_EXTENSION}, ""), nil
}

func OpenFileSegmentWithName(config SegmentConfig, name string) (*Segment, error) {

	filepath := filepath.Join(config.FileDirectory, name)

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

	// fmt.Println("Open segment", name, file.Name())

	return &Segment{
		Id:     0,
		File:   file,
		Writer: bufio.NewWriterSize(file, config.BufferSize),
		Offset: info.Size(),
		Closed: false,
	}, nil
}

func FindLastSegmentFromFilePaths(filepaths []string) (int, error) {
	last := 1

	for _, filepath := range filepaths {
		id, err := GetSegmentId(filepath)

		if err != nil {
			return 0, fmt.Errorf("Error getting segment id from file path: %w", err)
		}

		if id > last {
			last = id
		}

	}

	return last, nil
}

func DeleteStaleFileSegments(filepaths []string) error {

	for _, filepath := range filepaths {
		err := os.Remove(filepath)

		if err != nil {
			return fmt.Errorf("Error removing segment file: %s, error: %w", filepath, err)
		}
	}

	return nil
}

func RenameNewSegmentFiles(config SegmentConfig, filepaths []string, lastSegmentId int) (map[string]int, error) {
	lastId := lastSegmentId
	reversedFilePaths := filepaths
	// reverses the filepath so the latest segment random file comes first
	slices.Reverse(reversedFilePaths)

	randomFilePathMapping := make(map[string]int)

	for _, path := range reversedFilePaths {
		if lastId <= 0 {
			return nil, fmt.Errorf("Last segment id cannot be 0, Compaction failed.")
		}

		oldFilePath := filepath.Join(config.FileDirectory, path)
		newFilePath := filepath.Join(config.FileDirectory, GetSegmentName(lastId))
		err := os.Rename(oldFilePath, newFilePath)

		if err != nil {
			return nil, fmt.Errorf("Error renaming file segments, old: %s, new: %s, error: %w", oldFilePath, newFilePath, err)
		}

		randomFilePathMapping[path] = lastId
		lastId -= 1
	}

	return randomFilePathMapping, nil
}
