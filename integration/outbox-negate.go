package integration

import (
	"testing"
)

func RunOutboxNegateTests(t *testing.T) {
	t.Run("Negate", func(t *testing.T) {
		RunOutboxUndoTests(t)
	})
}

func RunOutboxUndoTests(t *testing.T) {
	t.Run("Undo", func(t *testing.T) {
		t.Skipf("TODO")
	})
}
