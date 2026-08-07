package variables

import "errors"

// file*
var FileNotFound = errors.New("File path is required")
var FileNotOpened = errors.New("Error opening the file")
var BufferSizeInvalid = errors.New("Buffer size invalid")

// storage
var KeyNotFound = errors.New("Key does not exist in the store")

// formatter
var EMPTY_KEY_RECORD = errors.New("Key cannot be empty")
