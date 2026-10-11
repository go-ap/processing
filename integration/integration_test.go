package integration

import (
	"testing"

	"github.com/go-ap/processing"
)

func Test(t *testing.T) {
	p := processing.New()

	if c2s {
		RunOutboxTests(t, p)
	}

	if s2s {
		RunInboxTests(t, p)
	}
}
