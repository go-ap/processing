package integration

import (
	"testing"
)

func RunOutboxTests(t *testing.T) {
	t.Run("Outbox", func(t *testing.T) {
		RunOutboxContentManagementTests(t)

		RunOutboxCollectionManagementTests(t)
		RunOutboxNegateTests(t)
	})
}
