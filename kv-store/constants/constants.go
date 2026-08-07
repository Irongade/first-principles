package constants

import "kvstore/types"

const (
	TEXT_FORMATTER types.FormatterOptions = 1
)

const (
	SyncNever types.SyncPolicy = iota
	SyncEveryWrite
)

const (
	PUT_OPERATION    types.Operation = "put"
	DELETE_OPERATION types.Operation = "del"
)

const (
	SegmentFileFormat types.FileFormat = "segment"
	SimpleFileFormat  types.FileFormat = "simple"
)

const TOMBSTONE = "-1"

const MaxRequestSize = 1 << 20

const SEGMENT_EXTENSION = ".log"
const SEGMENT_DIRECTORY = "./data"

const SEGMENT_PREFIX = "segment-"
const SEGMENT_NAME_WIDTH = 10

const FILE_PERMISSION = 0o600
const DIRECTORY_PERMISSION = 0o755

const TEXT_SEPARATOR = "|"
const NEW_LINE_SEPARATOR = "\n"
