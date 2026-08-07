package formatter

import "kvstore/types"

type Encoder interface {
	Encode(record types.Record) ([]byte, error)
}

type Decoder interface {
	Decode(data []byte) (types.Record, error)
}

type Formatter interface {
	Encoder
	Decoder
}
