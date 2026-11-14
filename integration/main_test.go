package integration

import (
	"io"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/spf13/pflag"
)

func init() {
	godog.BindCommandLineFlags("godog.", &opts)
}

var quiet bool

var opts = godog.Options{
	NoColors:      true,
	Format:        "progress",
	StopOnFailure: false,
}

func TestMain(m *testing.M) {
	pflag.BoolVarP(&quiet, "godog.quiet", "q", false, "Quiet all godog output.")
	pflag.Parse()

	if quiet {
		opts.Output = io.Discard
	}

	status := m.Run()
	os.Exit(status)
}
