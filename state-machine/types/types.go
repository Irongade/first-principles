package types

type Operation string

type Store interface {
	Get(K string) (string, error)
	Put(K string, V string) error
	Delete(K string) error
	Close() error
}
