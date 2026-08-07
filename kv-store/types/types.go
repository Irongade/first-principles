package types

type Operation string

type Record struct {
	Version   string
	Operation Operation
	Key       string
	Value     string
}

type SyncPolicy int
type RecordLocation struct {
	SegmentId int
	Offset    int64
	Size      uint32
}
type ScannedRecord struct {
	Record
	RecordLocation RecordLocation
}

type FileFormat string

type FormatterOptions int

type IndexFormat string
