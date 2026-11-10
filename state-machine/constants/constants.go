package constants

import (
	"errors"
	"os"
	"statemachine/types"
)

const (
	PUT_OPERATION    types.Operation = "put"
	DELETE_OPERATION types.Operation = "del"
)

const EntryHeaderSize = 24

const (
	LogDirName  = "log"
	DbDirName   = "db"
	LogFileName = "replication.log"
)

const DirectoryMode os.FileMode = 0o755
const LogFileMode os.FileMode = 0o644

// storage
var KeyNotFound = errors.New("Key does not exist in the store")
