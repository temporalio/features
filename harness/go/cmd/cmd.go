package cmd

import (
	"log"
	"os"

	"github.com/temporalio/features/harness/go/harness"
	"github.com/urfave/cli/v2"
)

// Execute executes the app using CLI arguments.
func Execute() {
	var err error

	// If the second arg is "go-subprocess", remove that part and run subprocess
	// app
	if len(os.Args) > 1 && os.Args[1] == "go-subprocess" {
		err = newSubprocessApp().Run(append([]string{os.Args[0]}, os.Args[2:]...))
	} else {
		err = newApp().Run(os.Args)
	}

	if err != nil {
		log.Fatal(err)
	}
}

// NewApp creates a new CLI app.
func newApp() *cli.App {
	return &cli.App{
		Commands: []*cli.Command{
			runCmd(),
		},
	}
}

func newSubprocessApp() *cli.App {
	return &cli.App{Commands: harness.GetRegisteredSubprocessCommands()}
}
