package integration

import (
	"testing"

	"github.com/go-ap/processing"
)

func RunOutboxTests(t *testing.T, P *processing.P) {
	t.Run("Outbox", func(t *testing.T) {
		RunOutboxContentManagementTests(t, P)

		RunOutboxNegateTests(t, P)

		t.Skipf("TODO")
	})
}

func RunOutboxContentManagementTests(t *testing.T, P *processing.P) {
	t.Run("Content Management", func(t *testing.T) {
		RunOutboxCreateTests(t, P)
		RunOutboxUpdateTests(t, P)
		RunOutboxDeleteTests(t, P)

		t.Skipf("TODO")
	})
}

func RunOutboxCreateTests(t *testing.T, P *processing.P) {
	t.Run("Create", func(t *testing.T) {
		t.Skipf("TODO")
	})
}

func RunOutboxUpdateTests(t *testing.T, P *processing.P) {
	t.Run("Update", func(t *testing.T) {
		t.Skipf("TODO")
	})
}

func RunOutboxDeleteTests(t *testing.T, P *processing.P) {
	t.Run("Delete", func(t *testing.T) {
		t.Skipf("TODO")
	})
}

func RunOutboxNegateTests(t *testing.T, P *processing.P) {
	t.Run("Negate", func(t *testing.T) {
		RunOutboxUndoTests(t, P)
		t.Skipf("TODO")
	})
}

func RunOutboxUndoTests(t *testing.T, P *processing.P) {
	t.Run("Undo", func(t *testing.T) {
		t.Skipf("TODO")
	})
}
