package jambon

import (
	"fmt"
	"strings"

	"github.com/b1naryth1ef/jambon/acmi"
	"github.com/b1naryth1ef/jambon/tacview"
	"github.com/urfave/cli/v2"
)

// CommandRecord handles recording TacView files from a real-time server
var CommandRecord = cli.Command{
	Name:        "record",
	Description: "record a tacview acmi file from a real time server",
	Action:      commandRecord,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "server",
			Usage:    "connection string for the TacView realtime server",
			Required: true,
		},
		&cli.PathFlag{
			Name:     "output",
			Usage:    "path to the output ACMI file",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "username",
			Usage: "username to use when connecting to the realtime server",
			Value: "jambon-record",
		},
		&cli.BoolFlag{
			Name:  "binary-format",
			Usage: "enable usage of the experimental ACMI binary format",
		},
	},
}

func commandRecord(ctx *cli.Context) error {
	serverStr := ctx.String("server")
	if !strings.Contains(serverStr, ":") {
		serverStr = fmt.Sprintf("%s:42674", serverStr)
	}

	reader, err := tacview.NewRealTimeReader(serverStr, ctx.String("username"))
	if err != nil {
		return err
	}

	outputFile, err := openWritableTacView(ctx.Path("output"))
	if err != nil {
		return err
	}

	var writer tacview.Writer
	if ctx.Bool("binary-format") {
		writer, err = acmi.NewWriter(outputFile, reader.Header())
	} else {
		writer, err = tacview.NewWriter(outputFile, reader.Header())
	}
	if err != nil {
		return err
	}

	defer writer.Close()

	data := make(chan *tacview.TimeFrame, 1)
	go reader.ProcessTimeFrames(1, data)

	for {
		frame, ok := <-data
		if !ok {
			break
		}

		err = writer.WriteTimeFrame(frame)
		if err != nil {
			return err
		}
	}

	return nil
}
