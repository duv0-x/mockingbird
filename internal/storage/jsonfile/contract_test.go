package jsonfile_test

import (
	"context"
	"testing"

	"github.com/duv0-x/mockingbird/internal/service/servicetest"
	"github.com/duv0-x/mockingbird/internal/storage"
	"github.com/duv0-x/mockingbird/internal/storage/jsonfile"
)

// TestStoreContract runs the contract shared with servicetest.MemStore.
func TestStoreContract(t *testing.T) {
	servicetest.StoreContract(t, func(t *testing.T) storage.Store {
		s, err := jsonfile.Open(context.Background(), jsonfile.Options{DataDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	})
}
