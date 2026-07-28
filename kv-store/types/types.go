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
	Offset int64
	Size   uint32
}

type ScannedRecord struct {
	Record
	RecordLocation RecordLocation
}
