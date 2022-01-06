package tacview

import (
	"fmt"
	"io"
	"strings"
)

type HeaderWriter interface {
	WriteHeader(*Header) error
}

type RawWriter interface {
	HeaderWriter

	Write(*RawTimeFrame) error
}

type ZWriter interface {
	HeaderWriter

	Write(*TimeFrame) error
}

type rawWriter struct {
	out io.Writer
}

func NewRawWriter(dest io.Writer) RawWriter {
	return &rawWriter{dest}
}

func (r *rawWriter) Write(tf *RawTimeFrame) error {
	data := []byte{}
	if tf.Offset != 0 {
		data = []byte(fmt.Sprintf("#%F\n", tf.Offset))
	}
	data = append(data, []byte(strings.Join(tf.Contents, "\n"))...)

	_, err := r.out.Write(append(data, '\n'))
	return err
}

// TODO
func (r *rawWriter) WriteHeader(header *Header) error {
	_, err := r.out.Write(bomHeader)
	if err != nil {
		return err
	}

	r.out.Write([]byte(fmt.Sprintf("FileType=%s\n", header.FileType)))
	r.out.Write([]byte(fmt.Sprintf("FileVersion=%s\n", header.FileVersion)))
	return r.Write(header.InitialTimeFrame.ToRaw())
}
