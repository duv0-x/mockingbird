package servicetest

import (
	"testing"

	"github.com/duv0-x/mockingbird/internal/storage"
)

func TestMemStoreContract(t *testing.T) {
	StoreContract(t, func(*testing.T) storage.Store { return NewMemStore() })
}
