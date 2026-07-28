package constants

import "kvstore/types"

const (
	SyncNever types.SyncPolicy = iota
	SyncEveryWrite
)

const (
	PUT_OPERATION    types.Operation = "put"
	DELETE_OPERATION types.Operation = "del"
)

const TOMBSTONE = "-1"

const MaxRequestSize = 1 << 20
