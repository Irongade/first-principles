package filewriter

import "errors"

var FileNotFound = errors.New("File path is required")
var FileNotOpened = errors.New("Error opening the file")
var BufferSizeInvalid = errors.New("Buffer size invalid")
