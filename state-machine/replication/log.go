package replication

import (
	"fmt"
	"os"
	"statemachine/encoding"
	"statemachine/file/simple"
	"statemachine/types"
	"sync"
)

type Log struct {
	mu sync.RWMutex
	// maps index -> where the log data is stored
	entries map[uint64]types.Location

	reader Reader
	writer Writer

	firstIndex uint64
	lastIndex  uint64

	closed bool
}

func CreateReplicatedLog(filePath string, filePermission os.FileMode) (*Log, error) {
	if filePath == "" {
		return nil, fmt.Errorf("File path cannot be empty")
	}

	if filePermission == 0 {
		return nil, fmt.Errorf("File permission cannot be empty")
	}

	writer, err := simple.CreateFileWriter(filePath, filePermission)

	if err != nil {
		return nil, fmt.Errorf("File writer not created")
	}

	reader, err := simple.CreateFileReader(filePath)

	if err != nil {
		return nil, fmt.Errorf("File reader not created")
	}

	entries, firstIndex, lastIndex, err := reader.RebuildIndex()

	if err != nil {
		return nil, fmt.Errorf("Error rebuilding index: %w", err)
	}

	return &Log{
		reader:     reader,
		writer:     writer,
		entries:    entries,
		firstIndex: firstIndex,
		lastIndex:  lastIndex,
		closed:     false,
	}, nil
}

func (l *Log) Append(data []byte) (types.LogEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return types.LogEntry{}, fmt.Errorf("Log is closed")
	}

	entry := types.LogEntry{
		Index: l.lastIndex + 1,
		Term:  0,
		Data:  data,
	}

	// encode the entry
	encoded, err := encoding.EncodeEntry(entry)

	if err != nil {
		return types.LogEntry{}, fmt.Errorf("Error encoding entry")
	}

	location, err := l.writer.Append(encoded)

	if err != nil {
		return types.LogEntry{}, fmt.Errorf("Error appending record to file: %w", err)
	}

	l.entries[entry.Index] = location
	l.lastIndex = entry.Index

	if l.firstIndex == 0 {
		l.firstIndex = entry.Index
	}

	return entry, nil
}

func (l *Log) Read(index uint64) (types.LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		return types.LogEntry{}, fmt.Errorf("Log is closed")
	}

	location, ok := l.entries[index]

	if !ok {
		return types.LogEntry{}, fmt.Errorf("Index entry not found")
	}

	data, err := l.reader.Read(location)

	if err != nil {
		return types.LogEntry{}, err
	}

	entry, err := encoding.DecodeEntry(data)

	if err != nil {
		return types.LogEntry{}, fmt.Errorf("Error decoding entry data: %w", err)
	}

	return entry, nil
}

func (l *Log) FirstIndex() uint64 {
	return l.firstIndex
}

func (l *Log) LastIndex() uint64 {
	return l.lastIndex
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return fmt.Errorf("Log already closed")
	}

	l.closed = true

	return nil
}
