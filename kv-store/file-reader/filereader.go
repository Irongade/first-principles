package filereader

import (
	"kvstore/types"
)

type FileReader interface {
	ReadAtOffset(location types.RecordLocation) (types.Record, error)
	ReadAll() ([]types.Record, error)
	ScanAll() ([]types.ScannedRecord, error)
	Close() error
	ScanStaleSegments() ([]types.ScannedRecord, []string, error)
}
