package integration

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
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
	Output:        colors.Colored(os.Stdout),
}

func parseGoFlags(osArgs []string, goFlagSet *flag.FlagSet) error {
	var skippedFlags []string
	appendNext := false
	for _, f := range osArgs {
		if strings.HasPrefix(f, "-test.") || appendNext {
			skippedFlags = append(skippedFlags, f)
			appendNext = f == "-test.run"
		}
	}
	return goFlagSet.Parse(skippedFlags)
}

func TestMain(m *testing.M) {
	pflag.BoolVarP(&quiet, "godog.quiet", "q", false, "Quiet all godog output.")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	parseGoFlags(os.Args[1:], flag.CommandLine)
	pflag.Parse()

	if quiet {
		opts.Output = io.Discard
	}

	status := m.Run()
	os.Exit(status)
}
