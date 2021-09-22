package jambon

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"

	"github.com/b1naryth1ef/jambon/acmi"
	"github.com/b1naryth1ef/jambon/tacview"
	"github.com/urfave/cli/v2"
)

const normalizeDescription = `Normalize an ACMI file by completely rewriting it. If the output file is zip encoded
 the internal ACMI text file will be placed in the root of the zip, ignoring any
 directory structure from the input file. Memory overhead can be massively reduced at
 the cost of speed by setting the concurrency flag to 1.`

// CommandNormalize handles rewriting ACMI files
var CommandNormalize = cli.Command{
	Name:        "normalize",
	Description: normalizeDescription,
	Action:      commandNormalize,
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
		&cli.StringSliceFlag{
			Name:  "exclude-property",
			Usage: "provide a key=value property pair that will cause matching objects to be excluded from the output",
		},
		&cli.IntFlag{
			Name:  "concurrency",
			Usage: "number of parallel processing routines to run",
			Value: runtime.GOMAXPROCS(-1),
		},
	},
}

func commandNormalize(ctx *cli.Context) error {
	excludeProperties := make(map[string]string)
	for _, property := range ctx.StringSlice("exclude-property") {
		parts := strings.SplitN(property, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("failed to process exclude property '%v'", property)
		}
		excludeProperties[parts[0]] = parts[1]
	}

	inputFile, err := openReadableTacView(ctx.Path("input"))
	if err != nil {
		return err
	}

	outputFile, err := openWritableTacView(ctx.Path("output"))
	if err != nil {
		return err
	}

	reader, err := acmi.NewReader(inputFile)
	if err != nil {
		return err
	}

	return normalize(ctx.Int("concurrency"), reader, outputFile, func(o *tacview.Object) bool {
		if len(excludeProperties) == 0 {
			return true
		}

		for k, v := range excludeProperties {
			prop := o.Get(k)
			if prop != nil && prop.Value == v {
				return false
			}
		}

		return true
	})
}

func normalize(concurrency int, input tacview.Reader, output io.WriteCloser, filter func(o *tacview.Object) bool) error {
	done := make(chan error)
	timeFrames := make(chan *tacview.TimeFrame)

	writer, err := tacview.NewWriter(output, input.Header())
	if err != nil {
		return err
	}
	defer writer.Close()

	collected := make([]*tacview.TimeFrame, 0)
	filteredObjects := make(map[uint64]struct{})
	go func() {
		defer close(done)

		for {
			tf, ok := <-timeFrames

			if !ok {
				return
			}

			for _, object := range tf.Objects {
				_, isFiltered := filteredObjects[object.Id]

				if object.Deleted && isFiltered {
					delete(filteredObjects, object.Id)
				} else if isFiltered {
					tf.Delete(object.Id)
				} else if !filter(object) {
					filteredObjects[object.Id] = struct{}{}
					tf.Delete(object.Id)
				}
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

	err = input.ProcessTimeFrames(concurrency, timeFrames)
	if err != nil {
		return err
	}

	err = <-done
	if err != nil {
		return err
	}

	if concurrency != 1 {
		sort.Slice(collected, func(i, j int) bool {
			return collected[i].Offset < collected[j].Offset
		})

		for _, tf := range collected {
			err = writer.WriteTimeFrame(tf)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
