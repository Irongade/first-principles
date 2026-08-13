package filewriter

import "kvstore/types"

type FileWriter interface {
	Append(record types.Record) (types.RecordLocation, error)
	Flush() error
	Close() error
	GetActiveSegmentId() (int, error)
}
