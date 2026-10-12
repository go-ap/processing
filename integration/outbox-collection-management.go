package integration

import (
	"testing"
)

func RunOutboxCollectionManagementTests(t *testing.T) {
	t.Run("Collection Management", func(t *testing.T) {
		RunOutboxAddTests(t)
		RunOutboxMoveTests(t)
		RunOutboxRemoveTests(t)
	})
}

func RunOutboxAddTests(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		t.Skipf("TODO")
	})
}

func RunOutboxMoveTests(t *testing.T) {
	t.Run("Move", func(t *testing.T) {
		t.Skipf("TODO")
	})
}

func RunOutboxRemoveTests(t *testing.T) {
	t.Run("Remove", func(t *testing.T) {
		t.Skipf("TODO")
	})
}
