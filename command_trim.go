package jambon

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"time"

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
		&cli.StringFlag{
			Name:  "start-at-time",
			Usage: "set the start point via a RFC3999 timestamp",
		},
		&cli.StringFlag{
			Name:  "end-at-time",
			Usage: "set the end point via a RFC3999 timestamp",
		},
		&cli.IntFlag{
			Name:  "concurrency",
			Usage: "number of parallel processing routines to run",
			Value: runtime.GOMAXPROCS(-1),
		},
	},
}

func commandTrim(ctx *cli.Context) error {
	inputFile, err := openReadableTacView(ctx.Path("input"))
	if err != nil {
		return err
	}

	outputFile, err := openWritableTacView(ctx.Path("output"))
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

		start = startTime.Sub(reader.Header().ReferenceTime).Seconds()
	}

	if ctx.IsSet("end-at-offset-time") {
		end = ctx.Float64("end-at-offset-time")
	} else if ctx.IsSet("end-at-time") {
		endTime, err := time.Parse("2006-01-02T15:04:05Z", ctx.String("start-at-time"))
		if err != nil {
			return err
		}

		end = endTime.Sub(reader.Header().ReferenceTime).Seconds()
	}

	return trimTimeFrame(ctx.Int("concurrency"), reader, outputFile, start, end)
}

func trimTimeFrame(concurrency int, reader tacview.Reader, dest io.WriteCloser, start float64, end float64) error {
	readerDone := make(chan error)
	done := make(chan error)
	timeFrames := make(chan *tacview.TimeFrame)

	go func() {
		defer close(readerDone)
		err := reader.ProcessTimeFrames(concurrency, timeFrames)
		if err != nil {
			readerDone <- err
		}
	}()

	// Stores any objects which where created before the start of our time window,
	// and have not yet been destroyed.
	preStartObjects := make(map[uint64]*tacview.Object)
	var firstTimeFrame *tacview.TimeFrame

	fmt.Printf("Scanning to offset %f...\n", start)
	for {
		tf, ok := <-timeFrames
		if !ok {
			return io.EOF
		}

		if tf.Offset >= start {
			firstTimeFrame = tf
			break
		}

		for _, object := range tf.Objects {
			existingObject := preStartObjects[object.Id]

			if existingObject == nil {
				preStartObjects[object.Id] = object
				continue
			} else if object.Deleted {
				delete(preStartObjects, object.Id)
			}

			for _, newProp := range object.Properties {
				existingObject.Set(newProp.Key, newProp.Value)
			}
		}
	}

	fmt.Printf("Collected %d active objects for frame 0\n", len(preStartObjects))

	referenceTime := reader.Header().ReferenceTime.Add(time.Second * time.Duration(start))

	// We copy the initial time frame completely
	initialTimeFrame := tacview.NewTimeFrame()
	initialTimeFrame.Offset = 0
	initialTimeFrame.Objects = reader.Header().InitialTimeFrame.Objects

	// We generate a frame 0 that contains any objects which existed at the start
	// point of our time window.
	preTimeFrame := tacview.NewTimeFrame()
	preTimeFrame.Offset = 0
	for _, object := range preStartObjects {
		preTimeFrame.Objects = append(firstTimeFrame.Objects, object)
	}

	header := &tacview.Header{
		FileType:         reader.Header().FileType,
		FileVersion:      reader.Header().FileVersion,
		ReferenceTime:    referenceTime,
		InitialTimeFrame: *initialTimeFrame,
	}

	writer, err := tacview.NewWriter(dest, header)
	if err != nil {
		return err
	}
	defer writer.Close()

	err = writer.WriteTimeFrame(preTimeFrame)
	if err != nil {
		return err
	}

	err = writer.WriteTimeFrame(firstTimeFrame)
	if err != nil {
		return err
	}

	collected := make([]*tacview.TimeFrame, 0)

	go func() {
		defer close(done)
		defer close(readerDone)

		for {
			tf, ok := <-timeFrames

			if !ok {
				return
			}

			if tf.Offset >= end {
				return
			}

			if concurrency == 1 {
				err := writer.WriteTimeFrame(tf)
				if err != nil {
					done <- err
					return
				}
			} else {
				collected = append(collected, tf)
			}
		}
	}()

	err = <-done
	if err != nil {
		return err
	}

	err = <-readerDone
	if err != nil {
		return err
	}

	if concurrency != 1 {
		fmt.Printf("Sorting %v collected frames...\n", len(collected))
		sort.Slice(collected, func(i, j int) bool {
			return collected[i].Offset < collected[j].Offset
		})

		fmt.Printf("Writing %v frames...\n", len(collected))
		for _, tf := range collected {
			tf.Offset = reader.Header().ReferenceTime.Add(time.Second * time.Duration(tf.Offset)).Sub(referenceTime).Seconds()
			err = writer.WriteTimeFrame(tf)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
