package fileutil

import (
	"fmt"
	"kvstore/constants"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SegmentConfig struct {
	FileDirectory string
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

	fmt.Println("file paths", validFiles)

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
