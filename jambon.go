package jambon

import (
	"io"
	"runtime"
	"sort"

	"github.com/b1naryth1ef/jambon/tacview"
)

type JambonProcessor interface {
	ProcessFile(*tacview.Reader) error
}

type JambonNoopProcessor struct {
	dest io.WriteCloser
}

func NewJambonNoopProcessor(dest io.WriteCloser) *JambonNoopProcessor {
	return &JambonNoopProcessor{dest: dest}
}

func (j *JambonNoopProcessor) ProcessFile(source *tacview.Reader) error {
	done := make(chan struct{})
	timeFrames := make(chan *tacview.TimeFrame)

	collected := make([]*tacview.TimeFrame, 0)

	go func() {
		defer close(done)

		for {
			tf, ok := <-timeFrames

			if !ok {
				return
			}

			collected = append(collected, tf)
		}
	}()

	err := source.ProcessTimeFrames(runtime.GOMAXPROCS(-1), timeFrames)
	if err != nil {
		return err
	}

	<-done

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Offset < collected[j].Offset
	})

	writer, err := tacview.NewWriter(j.dest, &source.Header)
	if err != nil {
		return err
	}
	defer writer.Close()

	for _, tf := range collected {
		err = writer.WriteTimeFrame(tf)
		if err != nil {
			return err
		}
	}

	return nil
}
