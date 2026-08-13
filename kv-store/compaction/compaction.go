package compaction

import (
	"errors"
	"fmt"
	"kvstore/constants"
	filereader "kvstore/file-reader"
	fileutil "kvstore/file-util"
	"kvstore/formatter"
	"kvstore/formatter/text"
	"kvstore/types"
	"sync"
)

type CompactorResult struct {
	Index                      map[string]types.ScannedRecord
	RecordToCompactedNameIndex map[string]string
	StaleFilepaths             []string
	LastSegmentId              int
	CompactedSegmentNames      []string
}

type Compactor struct {
	filereader filereader.FileReader
	rwLock     sync.RWMutex
	config     fileutil.SegmentConfig
	formatter  formatter.Formatter
}

func CreateNewCompactor(filereader filereader.FileReader, compactionConfig fileutil.SegmentConfig) (*Compactor, error) {
	if filereader == nil {
		return nil, fmt.Errorf("File reader cannot be nil")
	}

	var fmter formatter.Formatter

	switch compactionConfig.FormatOptions {
	case constants.TEXT_FORMATTER:
		fmter = &text.TextFormatter{}

	default:
		fmter = &text.TextFormatter{}
	}

	return &Compactor{
		filereader: filereader,
		formatter:  fmter,
		config:     compactionConfig,
	}, nil
}

func (c *Compactor) CompactStaleSegments() (CompactorResult, error) {
	c.rwLock.Lock()
	defer c.rwLock.Unlock()

	index := make(map[string]types.ScannedRecord)
	recordIndex := make(map[string]string)

	staleRecords, filepaths, err := c.filereader.ScanStaleSegments()

	if err != nil {
		return CompactorResult{}, fmt.Errorf("Failed to retrieve all stale records from stale segments, error: %w", err)
	}

	// nothing to compact
	if len(staleRecords) == 0 {
		return CompactorResult{}, nil
	}

	for _, record := range staleRecords {
		if record.Operation == constants.DELETE_OPERATION {
			delete(index, record.Key)
			continue
		}

		index[record.Key] = record
	}

	lastSegmentId, err := fileutil.FindLastSegmentFromFilePaths(filepaths)

	if err != nil {
		return CompactorResult{}, fmt.Errorf("Error retrieving last segment id: %d with error: %w", lastSegmentId, err)
	}

	compactedSegmentNames := make([]string, 0)

	currentSegment, currentSegmentName, err := c.CreateSegment()

	if err != nil {
		return CompactorResult{}, fmt.Errorf("Error creating segment for use.")
	}

	compactedSegmentNames = append(compactedSegmentNames, currentSegmentName)

	for key, value := range index {
		if currentSegment.Offset > int64(c.config.MaxSegmentSize) {
			newSegment, newSegmentName, err := c.CreateSegment()

			if err != nil {
				return CompactorResult{}, fmt.Errorf("Error creating segment for use during roll update. %w", err)
			}

			// close current segment
			flushErr := currentSegment.Writer.Flush()
			syncErr := currentSegment.File.Sync()
			closeErr := currentSegment.File.Close()

			if errors.Join(flushErr, syncErr, closeErr) != nil {
				return CompactorResult{}, fmt.Errorf("Error closing written segment during rolling update.")
			}

			currentSegment.Closed = true

			// create new segment
			currentSegment = newSegment
			currentSegmentName = newSegmentName
			compactedSegmentNames = append(compactedSegmentNames, newSegmentName)
		}

		encodedData, err := c.formatter.Encode(value.Record)
		encodedDataSize := len(encodedData)
		if err != nil {
			return CompactorResult{}, fmt.Errorf("Error encoding record during compaction: %w", err)
		}

		n, err := currentSegment.Writer.Write(encodedData)

		if err != nil {
			return CompactorResult{}, fmt.Errorf("Error appending record to segment: %w", err)
		}

		if encodedDataSize <= 0 || encodedDataSize != n {
			return CompactorResult{}, fmt.Errorf("Error in encoded data size: %s", value.Record.Key)
		}

		// record the current segment name for this new key
		recordIndex[value.Key] = currentSegmentName

		// update the offset and size of the value
		index[key] = types.ScannedRecord{
			Record: value.Record,
			RecordLocation: types.RecordLocation{
				// Segment should remain as is for now, it will be updated in the filestore layer
				SegmentId: value.RecordLocation.SegmentId,
				Offset:    currentSegment.Offset,
				Size:      uint32(encodedDataSize),
			},
		}

		// increment the offset for the segment
		currentSegment.Offset += int64(encodedDataSize)
	}

	return CompactorResult{
		Index:                      index,
		StaleFilepaths:             filepaths,
		LastSegmentId:              lastSegmentId,
		CompactedSegmentNames:      compactedSegmentNames,
		RecordToCompactedNameIndex: recordIndex,
	}, nil
}

func (c *Compactor) CreateSegment() (*fileutil.Segment, string, error) {

	randomName, err := fileutil.GetRandomSegmentName(7)

	if err != nil {
		return nil, "", fmt.Errorf("Error generating random segment name: %s with error: %w", randomName, err)
	}

	segment, err := fileutil.OpenFileSegmentWithName(c.config, randomName)

	if err != nil {
		return nil, "", fmt.Errorf("Error opening a new segment with random name: %s, with error: %w", randomName, err)
	}

	return segment, randomName, nil
}
