package jambon

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/b1naryth1ef/jambon/tacview"
	"github.com/urfave/cli/v2"
)

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
		&cli.StringFlag{
			Name:  "start-at-time",
			Usage: "set the start point via a RFC3999 timestamp",
		},
		&cli.StringFlag{
			Name:  "end-at-time",
			Usage: "set the end point via a RFC3999 timestamp",
		},
	},
}

func commandTrim(ctx *cli.Context) error {
	inputFile, err := os.Open(ctx.Path("input"))
	if err != nil {
		return err
	}

	outputFile, err := os.Create(ctx.Path("output"))
	if err != nil {
		return err
	}

	reader, err := tacview.NewReader(inputFile)
	if err != nil {
		return err
	}

	var start float64
	var end float64

	if ctx.IsSet("start-at-offset-time") {
		start = ctx.Float64("start-at-offset-time")
	} else if ctx.IsSet("start-at-time") {
		startTime, err := time.Parse("2006-01-02T15:04:05Z", ctx.String("start-at-time"))
		if err != nil {
			return err
		}

		start = startTime.Sub(reader.Header.ReferenceTime).Seconds()
	}

	if ctx.IsSet("end-at-offset-time") {
		end = ctx.Float64("end-at-offset-time")
	} else if ctx.IsSet("end-at-time") {
		endTime, err := time.Parse("2006-01-02T15:04:05Z", ctx.String("start-at-time"))
		if err != nil {
			return err
		}

		end = endTime.Sub(reader.Header.ReferenceTime).Seconds()
	}

	return trimTimeFrame(reader, outputFile, start, end)
}

func trimTimeFrame(reader *tacview.Reader, dest io.WriteCloser, start float64, end float64) error {
	done := make(chan struct{})
	timeFrames := make(chan *tacview.TimeFrame)

	objects := make(map[uint64]*tacview.Object)

	collected := make([]*tacview.TimeFrame, 0)

	go func() {
		defer close(done)

		for {
			tf, ok := <-timeFrames

			if !ok {
				return
			}

			if tf.Offset < start {
				for _, object := range tf.Objects {
					existingObject := objects[object.Id]

					if existingObject == nil {
						objects[object.Id] = object
						continue
					}

					for _, newProp := range object.Properties {
						existingObject.Set(newProp.Key, newProp.Value)
					}
				}
			}

			if tf.Offset >= start && tf.Offset <= end {
				collected = append(collected, tf)
			}
		}
	}()

	fmt.Printf("Collecting frames between %v and %v...\n", start, end)
	err := reader.ProcessTimeFrames(runtime.GOMAXPROCS(-1), timeFrames)
	if err != nil {
		return err
	}

	<-done

	if len(collected) == 0 {
		return fmt.Errorf("No collected frames, is your time range valid?")
	}

	fmt.Printf("Sorting %v collected frames...\n", len(collected))
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Offset < collected[j].Offset
	})

	referenceTime := reader.Header.ReferenceTime.Add(time.Second * time.Duration(collected[0].Offset))
	initialTimeFrame := tacview.NewTimeFrame()
	initialTimeFrame.Offset = 0
	initialTimeFrame.Objects = reader.Header.InitialTimeFrame.Objects

	firstTimeFrame := tacview.NewTimeFrame()
	firstTimeFrame.Offset = 0
	for _, object := range objects {
		firstTimeFrame.Objects = append(firstTimeFrame.Objects, object)
	}

	header := &tacview.Header{
		FileType:         reader.Header.FileType,
		FileVersion:      reader.Header.FileVersion,
		ReferenceTime:    referenceTime,
		InitialTimeFrame: *initialTimeFrame,
	}

	fmt.Printf("Writing %v frames...\n", len(collected))
	writer, err := tacview.NewWriter(dest, header)
	if err != nil {
		return err
	}
	defer writer.Close()

	err = writer.WriteTimeFrame(firstTimeFrame)
	if err != nil {
		return err
	}

	for _, tf := range collected {
		tf.Offset = reader.Header.ReferenceTime.Add(time.Second * time.Duration(tf.Offset)).Sub(referenceTime).Seconds()
		err = writer.WriteTimeFrame(tf)
		if err != nil {
			return err
		}
	}

	return nil
}
