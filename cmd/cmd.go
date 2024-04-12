package cmd

import (
	"log"
	"os"

	"github.com/temporalio/features/harness/go/cmd"
	"github.com/urfave/cli/v2"
)

// Execute executes the default app using CLI arguments.
func Execute() {
	// As a special case, if go-subprocess is the second arg, we need to forward
	// to harness command directly
	if len(os.Args) > 1 && os.Args[1] == "go-subprocess" {
		cmd.Execute()
		return
	}

	if err := NewApp().Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// NewApp creates a new CLI app.
func NewApp() *cli.App {
	return &cli.App{
		Commands: []*cli.Command{
			prepareCmd(),
			runCmd(),
			buildImageCmd(),
			publishImageCmd(),
		},
	}
}
