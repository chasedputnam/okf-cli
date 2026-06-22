// Command okf-cli converts documentation into Open Knowledge Format bundles.
package main

import (
	"os"

	"github.com/chasedputnam/okf-cli/internal/cli"
)

// Version is set at build time via ldflags.
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
