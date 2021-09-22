package jambon

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/b1naryth1ef/jambon/acmi"
	"github.com/b1naryth1ef/jambon/tacview"
	"github.com/urfave/cli/v2"
)

// CommandSearch handles searching a tacview for objects with a given set of properties
var CommandSearch = cli.Command{
	Name:        "search",
	Description: "search for an object",
	Action:      commandSearch,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:      "file",
			Usage:     "path to tacview files you'd like to search",
			TakesFile: true,
			Required:  true,
		},
		&cli.StringSliceFlag{
			Name:  "property",
			Usage: "provide a key=value property pair to search for",
		},
		&cli.BoolFlag{
			Name:  "print-properties",
			Usage: "print found object properties",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "output data as JSON",
		},
		&cli.IntFlag{
			Name:  "concurrency",
			Usage: "number of parallel processing routines to run",
			Value: runtime.GOMAXPROCS(-1),
		},
	},
}

func commandSearch(ctx *cli.Context) error {
	properties := make(map[string]string)
	for _, property := range ctx.StringSlice("property") {
		parts := strings.SplitN(property, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("failed to process property '%v'", property)
		}
		properties[parts[0]] = parts[1]
	}

	if len(properties) == 0 {
		return fmt.Errorf("no properties to search for")
	}

	for _, filePath := range ctx.StringSlice("file") {
		fmt.Fprintf(os.Stderr, "Processing file %v...\n", filePath)

		file, err := openReadableTacView(filePath)
		if err != nil {
			return err
		}

		reader, err := acmi.NewReader(file)
		if err != nil {
			return err
		}

		results, err := search(ctx.Int("concurrency"), reader, properties)
		if err != nil {
			return err
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].Object.Id < results[j].Object.Id
		})
		if ctx.Bool("json") {
			encoded, err := json.Marshal(results)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(encoded))
		} else {
			for _, result := range results {
				firstSeenDate := reader.Header().ReferenceTime.Add(time.Second * time.Duration(result.FirstSeen))
				lastSeenDate := reader.Header().ReferenceTime.Add(time.Second * time.Duration(result.LastSeen))

				fmt.Printf(
					"Object %v\n  First Seen: %v (%v)\n  Last Seen:  %v (%v)\n",
					result.Object.Id,
					firstSeenDate.Format(time.RFC3339),
					result.FirstSeen,
					lastSeenDate.Format(time.RFC3339),
					result.LastSeen,
				)
				if ctx.Bool("print-properties") {
					for _, property := range result.Object.Properties {
						fmt.Printf("  %v = %v\n", property.Key, property.Value)
					}
				}
			}

		}

	}

	return nil
}

type searchResult struct {
	Object    *tacview.Object `json:"object"`
	FirstSeen float64         `json:"first_seen"`
	LastSeen  float64         `json:"last_seen"`
}

func search(concurrency int, reader tacview.Reader, properties map[string]string) ([]*searchResult, error) {
	done := make(chan struct{})
	timeFrames := make(chan *tacview.TimeFrame)

	results := make(map[uint64]*searchResult)

	go func() {
		defer close(done)

		for {
			tf, ok := <-timeFrames

			if !ok {
				return
			}

			for _, object := range tf.Objects {
				if _, ok := results[object.Id]; !ok {
					matched := true
					for k, v := range properties {
						if res := object.Get(k); res == nil || res.Value != v {
							matched = false
							break
						}
					}

					if matched {
						results[object.Id] = &searchResult{Object: object, FirstSeen: tf.Offset, LastSeen: tf.Offset}
					}
				} else {
					result := results[object.Id]
					if tf.Offset < result.FirstSeen {
						result.FirstSeen = tf.Offset
					}

					if tf.Offset > result.LastSeen {
						result.LastSeen = tf.Offset
					}
				}
			}
		}
	}()

	err := reader.ProcessTimeFrames(concurrency, timeFrames)
	if err != nil {
		return nil, err
	}

	<-done

	final := make([]*searchResult, len(results))
	idx := 0
	for _, result := range results {
		final[idx] = result
		idx++
	}

	return final, nil
}
