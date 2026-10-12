package integration

import (
	"flag"
	"os"
	"testing"
)

var (
	verbose bool
	outbox  bool
	inbox   bool
)

func TestMain(m *testing.M) {
	flag.BoolVar(&verbose, "verbose", false, "enable more verbose logging")
	flag.BoolVar(&outbox, "outbox", false, "enable testing activities received in an actor's Outbox")
	flag.BoolVar(&inbox, "inbox", false, "enable testing activities received in an actor's Inbox")
	flag.Parse()

	if st := m.Run(); st != 0 {
		os.Exit(st)
	}
}
