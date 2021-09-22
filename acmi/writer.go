package acmi

import (
	"bufio"
	"io"

	"github.com/b1naryth1ef/jambon/tacview"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type Writer struct {
	writer *bufio.Writer
	closer io.Closer
}

func NewWriter(writer io.WriteCloser, header *tacview.Header) (*Writer, error) {
	w := &Writer{
		writer: bufio.NewWriter(writer),
		closer: writer,
	}
	return w, w.WriteTimeFrame(&header.InitialTimeFrame)
}

func (w *Writer) WriteTimeFrame(tf *tacview.TimeFrame) error {
	objects := make([]*Object, len(tf.Objects))
	for idx, object := range tf.Objects {
		objects[idx] = &Object{
			Id:         uint32(object.Id),
			Properties: make([]*Property, 0),
		}

		if object.Deleted {
			objects[idx].Delete = true
		}

		for _, property := range object.Properties {
			if property.Key == "T" {
				position, err := tacview.ReadObjectPosition(property.Value)
				if err != nil {
					return err
				}

				objects[idx].Position = &Position{
					Latitude:  position.Latitude,
					Longitude: position.Longitude,
					Altitude:  position.Altitude,
					U:         position.U,
					V:         position.V,
					Pitch:     position.Pitch,
					Roll:      position.Roll,
					Yaw:       position.Yaw,
					Heading:   position.Heading,
				}
				continue
			}

			objects[idx].Properties = append(objects[idx].Properties,
				&Property{
					Key:   property.Key,
					Value: property.Value,
				})
		}

		// TODO: object position
	}

	protoTimeFrame := &TimeFrame{
		Offset:  tf.Offset,
		Objects: objects,
	}

	data, err := proto.Marshal(protoTimeFrame)
	if err != nil {
		return err
	}

	buff := append(protowire.AppendVarint([]byte{}, uint64(len(data))), data...)
	_, err = w.writer.Write(buff)
	return err
}

func (w *Writer) Close() error {
	return w.closer.Close()
}
