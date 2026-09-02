package encoding

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"statemachine/constants"
	"statemachine/types"
)

// checksum -> index no -> term -> data length ---> data
func EncodeEntry(entry types.LogEntry) ([]byte, error) {

	buf := make([]byte, constants.EntryHeaderSize+len(entry.Data))

	binary.BigEndian.PutUint64(buf[4:12], entry.Index)
	binary.BigEndian.PutUint64(buf[12:20], entry.Term)
	binary.BigEndian.PutUint32(buf[20:24], uint32(len(entry.Data)))

	copy(buf[24:], entry.Data)

	checksum := crc32.ChecksumIEEE(buf[4:])
	binary.BigEndian.PutUint32(buf[0:4], checksum)

	return buf, nil

}

func DecodeEntry(buf []byte) (types.LogEntry, error) {
	if len(buf) < constants.EntryHeaderSize {
		return types.LogEntry{}, fmt.Errorf("Data cannot be smaller than header")
	}

	storedChecksum := binary.BigEndian.Uint32(buf[0:4])
	index := binary.BigEndian.Uint64(buf[4:12])
	term := binary.BigEndian.Uint64(buf[12:20])
	dataLength := binary.BigEndian.Uint32(buf[20:24])

	expectedLen := constants.EntryHeaderSize + int(dataLength)

	if expectedLen != len(buf) {
		return types.LogEntry{}, fmt.Errorf("Bad data length")
	}

	actualChecksum := crc32.ChecksumIEEE(buf[4:])

	if storedChecksum != actualChecksum {
		return types.LogEntry{}, fmt.Errorf("Checksums do not match, possible corrupted data")
	}

	data := make([]byte, dataLength)
	copy(data, buf[constants.EntryHeaderSize:])

	return types.LogEntry{
		Index: index,
		Term:  term,
		Data:  data,
	}, nil
}
