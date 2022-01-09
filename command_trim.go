package jambon

import (
	"os"
	"runtime/pprof"

	"github.com/b1naryth1ef/jambon/tacview"
	"github.com/urfave/cli/v2"
)

// CommandTrim handles trimming a tacview file
var CommandTrim = cli.Command{
	Name:        "trim",
	Description: "trim a tacview to reduce its duration",
	Action:      commandTrim,
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:     "input",
			Usage:    "path to the input ACMI file",
			Required: true,
		},
		&cli.PathFlag{
			Name:     "output",
			Usage:    "path to the output ACMI file",
			Required: true,
		},
		&cli.Float64Flag{
			Name:  "start-at-offset-time",
			Usage: "set the start point via an offset time",
		},
		&cli.Float64Flag{
			Name:  "end-at-offset-time",
			Usage: "set the end point via an offset time",
		},
		&cli.PathFlag{
			Name:  "cpuprofile",
			Usage: "record a cpu profile for debugging purposes",
		},
	},
}

func commandTrim(ctx *cli.Context) error {
	cpuprofile := ctx.Path("cpuprofile")
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	inputFile, err := openReadableTacView(ctx.Path("input"))
	if err != nil {
		return err
	}

	outputFile, err := openWritableTacView(ctx.Path("output"))
	if err != nil {
		return err
	}
	defer outputFile.Close()

	var start float64
	var end float64

	if ctx.IsSet("start-at-offset-time") {
		start = ctx.Float64("start-at-offset-time")
	}

	if ctx.IsSet("end-at-offset-time") {
		end = ctx.Float64("end-at-offset-time")
	}

	parser, err := tacview.NewParser(inputFile)
	if err != nil {
		return err
	}
	return tacview.TrimRaw(parser, tacview.NewRawWriter(outputFile), start, end)
}
