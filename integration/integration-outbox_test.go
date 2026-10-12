package integration

import (
	"testing"
)

func Test_Outbox(t *testing.T) {
	if !outbox {
		return
	}

	RunOutboxTests(t)
}
