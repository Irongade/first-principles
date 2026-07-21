package text

import (
	"bytes"
	"fmt"
	"kvstore/constants"
	"kvstore/types"
	"strings"
)

func (tf *TextFormatter) Decode(data []byte) (types.Record, error) {
	data = bytes.TrimSuffix(data, []byte(TEXT_SEPARATOR))

	parts := strings.SplitN(string(data), TEXT_SEPARATOR, 4)

	if len(parts) != 4 {
		return types.Record{}, fmt.Errorf("invalid record: expected 4 fields, got: %d", len(parts))
	}

	record := types.Record{
		Version:   parts[0],
		Operation: types.Operation(parts[1]),
		Key:       parts[2],
		Value:     parts[3],
	}

	if record.Operation != constants.PUT_OPERATION &&
		record.Operation != constants.DELETE_OPERATION {
		return types.Record{}, fmt.Errorf("Invalid Operation type")
	}

	if record.Key == "" {
		return types.Record{}, fmt.Errorf("Record Key cannot be empty")
	}

	return record, nil
}
