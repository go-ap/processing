package integration

import (
	"testing"
)

func RunInboxTests(t *testing.T) {
	t.Run("Inbox", func(t *testing.T) {
		RunInboxContentManagementTests(t)
	})
}
