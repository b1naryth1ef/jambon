package acmi

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/b1naryth1ef/jambon/tacview"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type Reader struct {
	reader *bufio.Reader
	header *tacview.Header
}

// NewReader creates a new ACMI reader
func NewReader(reader io.Reader) (*Reader, error) {
	r := &Reader{reader: bufio.NewReader(reader)}
	initialTimeFrame, err := r.readTimeFrame()
	if err != nil {
		return nil, err
	}
	globalObj := initialTimeFrame.Get(0)
	if globalObj == nil {
		return nil, fmt.Errorf("no global object found in initial time frame")
	}

	referenceTimeProperty := globalObj.Get("ReferenceTime")
	if referenceTimeProperty == nil {
		return nil, fmt.Errorf("global object is missing ReferenceTime")
	}

	referenceTime, err := time.Parse("2006-01-02T15:04:05Z", referenceTimeProperty.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ReferenceTime: `%v`", referenceTimeProperty.Value)
	}

	r.header = &tacview.Header{InitialTimeFrame: *initialTimeFrame}
	r.header.ReferenceTime = referenceTime
	return r, nil
}

func (r *Reader) Header() *tacview.Header {
	return r.header
}

func (r *Reader) ProcessTimeFrames(concurrency int, timeFrame chan<- *tacview.TimeFrame) error {
	for {
		tf, err := r.readTimeFrame()
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				close(timeFrame)
				return nil
			}
			return err
		}

		timeFrame <- tf
	}
}

func (r *Reader) readTimeFrame() (*tacview.TimeFrame, error) {
	peek, err := r.reader.Peek(8)
	if err != nil {
		return nil, err
	}

	tfSize, n := protowire.ConsumeVarint(peek)
	data := make([]byte, uint64(n)+tfSize)
	_, err = io.ReadFull(r.reader, data)
	if err != nil {
		return nil, err
	}

	var tf TimeFrame
	err = proto.Unmarshal(data[n:], &tf)
	if err != nil {
		return nil, err
	}

	outTf := &tacview.TimeFrame{}
	outTf.Offset = tf.Offset
	outTf.Objects = make([]*tacview.Object, len(tf.Objects))

	for idx, object := range tf.Objects {
		outTf.Objects[idx] = &tacview.Object{
			Id:         uint64(object.Id),
			Deleted:    object.Delete,
			Properties: make([]*tacview.Property, len(object.Properties)),
		}

		if object.Position != nil {
			outTf.Objects[idx].Properties = append(outTf.Objects[idx].Properties, &tacview.Property{Key: "T", Value: PositionToString(object.Position)})
		}

		for propIdx, prop := range object.Properties {
			outTf.Objects[idx].Properties[propIdx] = &tacview.Property{
				Key:   prop.Key,
				Value: prop.Value,
			}
		}
	}

	return outTf, nil
}

func PositionToString(o *Position) string {
	if o.U == 0 &&
		o.V == 0 &&
		o.Roll == 0 &&
		o.Pitch == 0 &&
		o.Yaw == 0 &&
		o.Heading == 0 {
		return fmt.Sprintf("%F|%F|%F", o.Longitude, o.Latitude, o.Altitude)
	} else if o.Roll == 0 &&
		o.Pitch == 0 &&
		o.Yaw == 0 &&
		o.Heading == 0 {
		return fmt.Sprintf("%F|%F|%F|%F|%F", o.Longitude, o.Latitude, o.Altitude, o.U, o.V)
	} else if o.U == 0 && o.V == 0 {
		return fmt.Sprintf("%F|%F|%F|%F|%F|%F", o.Longitude, o.Latitude, o.Altitude, o.Roll, o.Pitch, o.Yaw)
	} else {
		return fmt.Sprintf("%F|%F|%F|%F|%F|%F|%F|%F|%F", o.Longitude, o.Latitude, o.Altitude, o.Roll, o.Pitch, o.Yaw, o.U, o.V, o.Heading)
	}
}
