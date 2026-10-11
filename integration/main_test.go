package integration

import (
	"flag"
	"os"
	"testing"
)

var (
	verbose bool
	c2s     bool
	s2s     bool
)

func TestMain(m *testing.M) {
	flag.BoolVar(&verbose, "verbose", false, "enable more verbose logging")
	flag.BoolVar(&c2s, "c2s", false, "enable testing client to server functionality")
	flag.BoolVar(&s2s, "s2s", false, "enable testing server to server functionality")
	flag.Parse()

	if st := m.Run(); st != 0 {
		os.Exit(st)
	}
}
