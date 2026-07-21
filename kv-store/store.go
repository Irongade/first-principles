package kvstore

type Store interface {
	// get retrieves a string value or returns an error
	Get(K string) (string, error)

	// Put updates a value or returns an error
	Put(K string, V string) error

	// Delete returns an error
	Delete(K string) error

	// closes the DB
	Close() error
}

type kvstore struct {
	store Store
}

func New() {

}
