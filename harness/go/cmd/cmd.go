package cmd

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

// Execute executes the app using CLI arguments.
func Execute() {
	if err := NewApp().Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// NewApp creates a new CLI app.
func NewApp() *cli.App {
	return &cli.App{
		Commands: []*cli.Command{
			runCmd(),
		},
	}
}
