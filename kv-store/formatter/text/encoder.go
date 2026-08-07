package text

import (
	"kvstore/constants"
	"kvstore/types"
	"kvstore/variables"
	"strings"
)

func (tf *TextFormatter) Encode(record types.Record) ([]byte, error) {
	if record.Version == "" {
		return nil, variables.EMPTY_KEY_RECORD
	}

	if record.Key == "" {
		return nil, variables.EMPTY_KEY_RECORD
	}

	var value string
	switch record.Operation {
	case constants.PUT_OPERATION:
		value = record.Value

	case constants.DELETE_OPERATION:
		value = constants.TOMBSTONE

	}

	fields := []string{
		string(record.Version),
		string(record.Operation),
		escapeField(record.Key),
		escapeField(value),
	}

	appendValue := strings.Join(fields, constants.TEXT_SEPARATOR) + "\n"

	return []byte(appendValue), nil
}

func escapeField(value string) string {
	var builder strings.Builder

	for _, char := range value {
		switch char {
		case '\\':
			builder.WriteString(`\\`)
		case '|':
			builder.WriteString(`\|`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		default:
			builder.WriteRune(char)
		}
	}

	return builder.String()
}
