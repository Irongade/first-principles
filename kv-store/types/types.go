package types

type Operation string

type Record struct {
	Version   string
	Operation Operation
	Key       string
	Value     string
}

type SyncPolicy int
