package main

import (
	"log"
	"os"

	"github.com/b1naryth1ef/jambon"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "jambon",
		Description: "slims up those piggy acmi files",
		Commands: []*cli.Command{
			&jambon.CommandSearch,
			&jambon.CommandTrim,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
