package integration

import (
	"testing"
)

func Test_Inbox(t *testing.T) {
	if !inbox {
		return
	}

	RunInboxTests(t)
}
