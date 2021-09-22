package tacview

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spkg/bom"
)

var bomHeader = []byte{0xef, 0xbb, 0xbf}
var objectLineRe = regexp.MustCompile(`^(-?[0-9a-fA-F]+)(?:,((?:.|\n)*)+)?`)
var keyRe = regexp.MustCompilePOSIX("^(.*)=(.*)$")

func splitPropertyTokens(s string) (tokens []string, err error) {
	var runes []rune
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			inEscape = false
			fallthrough
		default:
			runes = append(runes, r)
		case r == '\\':
			inEscape = true
		case r == ',':
			tokens = append(tokens, string(runes))
			runes = runes[:0]
		}
	}
	tokens = append(tokens, string(runes))
	if inEscape {
		err = errors.New("invalid escape")
	}
	return tokens, err
}

// Header describes a ACMI file header
type Header struct {
	FileType         string
	FileVersion      string
	ReferenceTime    time.Time
	InitialTimeFrame TimeFrame
}

type Reader interface {
	ProcessTimeFrames(concurrency int, timeFrame chan<- *TimeFrame) error
	Header() *Header
}

// Reader provides an interface for reading an ACMI file
type reader struct {
	header Header
	reader *bufio.Reader
}

type Writer interface {
	io.Closer
	WriteTimeFrame(*TimeFrame) error
}

// Writer provides an interface for writing an ACMI file
type writer struct {
	writer *bufio.Writer
	closer io.Closer
}

// TimeFrame represents a single time frame from an ACMI file
type TimeFrame struct {
	Offset  float64
	Objects []*Object
}

// Property represents an object property
type Property struct {
	Key   string
	Value string
}

// Object describes an ACMI object
type Object struct {
	Id         uint64
	Properties []*Property
	Deleted    bool
}

// NewTimeFrame creates an empty TimeFrame
func NewTimeFrame() *TimeFrame {
	return &TimeFrame{
		Objects: make([]*Object, 0),
	}
}

// NewWriter creates a new ACMI writer
func NewWriter(wr io.WriteCloser, header *Header) (Writer, error) {
	w := &writer{
		writer: bufio.NewWriter(wr),
		closer: wr,
	}
	return w, w.writeHeader(header)
}

// NewReader creates a new ACMI reader
func NewReader(rdr io.Reader) (Reader, error) {
	r := &reader{reader: bufio.NewReader(bom.NewReader(rdr))}
	err := r.readHeader()
	return r, err
}

// Close closes the writer, flushing any remaining contents
func (w *writer) Close() error {
	err := w.writer.Flush()
	if err != nil {
		return err
	}
	return w.closer.Close()
}

func (w *writer) writeHeader(header *Header) error {
	_, err := w.writer.Write(bomHeader)
	if err != nil {
		return err
	}

	return header.Write(w.writer)
}

// WriteTimeFrame writes a time frame
func (w *writer) WriteTimeFrame(tf *TimeFrame) error {
	return tf.Write(w.writer, true)
}

func (h *Header) Write(writer *bufio.Writer) error {
	_, err := writer.WriteString("FileType=text/acmi/tacview\nFileVersion=2.2\n")
	if err != nil {
		return err
	}

	h.InitialTimeFrame.Write(writer, false)

	return writer.Flush()
}

// Get returns an object (if one exists) for a given object id
func (tf *TimeFrame) Get(id uint64) *Object {
	for _, object := range tf.Objects {
		if object.Id == id {
			return object
		}
	}
	return nil
}

// Delete removes an object (if one exists) for a given object id
func (tf *TimeFrame) Delete(id uint64) {
	for idx, object := range tf.Objects {
		if object.Id == id {
			tf.Objects = append(tf.Objects[:idx], tf.Objects[idx+1:]...)
			break
		}
	}
}

func (tf *TimeFrame) Write(writer *bufio.Writer, includeOffset bool) error {
	if includeOffset {
		_, err := writer.WriteString(fmt.Sprintf("#%F\n", tf.Offset))
		if err != nil {
			return err
		}
	}

	for _, object := range tf.Objects {
		object.Write(writer)
	}

	return nil
}

// Set updates the given property
func (o *Object) Set(key string, value string) {
	for _, property := range o.Properties {
		if property.Key == key {
			property.Value = value
			return
		}
	}
	o.Properties = append(o.Properties, &Property{Key: key, Value: value})
}

// Get returns a property (if one exists) for a given key
func (o *Object) Get(key string) *Property {
	for _, property := range o.Properties {
		if property.Key == key {
			return property
		}
	}
	return nil
}

func (o *Object) Write(writer *bufio.Writer) error {
	if o.Deleted {
		_, err := writer.WriteString(fmt.Sprintf("-%x\n", o.Id))
		return err
	}

	_, err := writer.WriteString(fmt.Sprintf("%x", o.Id))
	if err != nil {
		return err
	}

	if len(o.Properties) == 0 {
		_, err = writer.WriteString(",\n")
		return err
	}

	for _, property := range o.Properties {
		_, err = writer.WriteString(fmt.Sprintf(
			",%s=%s",
			property.Key,
			strings.Replace(strings.Replace(property.Value, "\n", "\\\n", -1), ",", "\\,", -1)),
		)
		if err != nil {
			return err
		}
	}

	_, err = writer.WriteString("\n")
	return err
}

func (r *reader) Header() *Header {
	return &r.header
}

func (r *reader) parseObject(object *Object, data string) error {
	parts, err := splitPropertyTokens(data)
	if err != nil {
		return err
	}

	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		partSplit := strings.SplitN(part, "=", 2)
		if len(partSplit) != 2 {
			return fmt.Errorf("Failed to parse property: `%v`", part)
		}

		object.Properties = append(object.Properties, &Property{Key: partSplit[0], Value: partSplit[1]})
	}

	return nil
}

// ProcessTimeFrames concurrently processes time frames from within the ACMI file,
//  producing them to an output channel. If your use case requires strong ordering
//  and you do not wish to implement this guarantee on the consumer side, you must
//  set the concurrency to 1.
func (r *reader) ProcessTimeFrames(concurrency int, timeFrame chan<- *TimeFrame) error {
	bufferChan := make(chan []byte)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				data, ok := <-bufferChan
				if data == nil || !ok {
					return
				}

				tf := NewTimeFrame()
				err := r.parseTimeFrame(data, tf, true)
				if err != nil && err != io.EOF {
					fmt.Printf("Failed to process time frame: (%v) %v\n", string(data), err)
					close(timeFrame)
					return
				}

				timeFrame <- tf
			}
		}()
	}

	err := r.timeFrameProducer(bufferChan)
	for i := 0; i < concurrency; i++ {
		bufferChan <- nil
	}

	wg.Wait()
	close(timeFrame)
	return err
}

func (r *reader) timeFrameProducer(buffs chan<- []byte) error {
	var buf []byte
	for {
		line, err := r.reader.ReadBytes('\n')
		if err == io.EOF {
			buffs <- buf
			return nil
		} else if err != nil {
			return err
		}

		if line[0] != '#' {
			buf = append(buf, line...)
			continue
		}

		if len(buf) > 0 {
			buffs <- buf
		}
		buf = line
	}
}

func (r *reader) parseTimeFrame(data []byte, timeFrame *TimeFrame, parseOffset bool) error {
	reader := bufio.NewReader(bytes.NewBuffer(data))
	return r.readTimeFrame(reader, timeFrame, parseOffset)
}

func (r *reader) readTimeFrame(reader *bufio.Reader, timeFrame *TimeFrame, parseOffset bool) error {
	if parseOffset {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		if len(line) == 0 || line[0] != '#' {
			return fmt.Errorf("Expected time frame offset, found `%v`", line)
		}

		offset, err := strconv.ParseFloat(line[1:len(line)-1], 64)
		if err != nil {
			return err
		}

		timeFrame.Offset = offset
	}

	timeFrameObjectCache := make(map[uint64]*Object)

	for {
		buffer := ""

		nextLinePrefix, err := reader.Peek(1)
		if err != nil {
			return err
		}

		if nextLinePrefix[0] == '#' {
			break
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return err
			}

			buffer = buffer + strings.TrimSuffix(line, "\n")
			if !strings.HasSuffix(buffer, "\\") {
				break
			}

			buffer = buffer[:len(buffer)-1] + "\n"
		}

		rawLineParts := objectLineRe.FindAllStringSubmatch(buffer, -1)
		if len(rawLineParts) != 1 {
			return fmt.Errorf("Failed to parse line: `%v` (%v)", buffer, len(rawLineParts))
		}

		lineParts := rawLineParts[0]

		if lineParts[1][0] == '-' {
			objectId, err := strconv.ParseUint(lineParts[1][1:], 16, 64)
			if err != nil {
				return err
			}

			if timeFrameObjectCache[objectId] != nil {
				timeFrameObjectCache[objectId].Deleted = true
			} else {
				object := &Object{Id: objectId, Properties: make([]*Property, 0), Deleted: true}
				timeFrameObjectCache[objectId] = object
				timeFrame.Objects = append(timeFrame.Objects, object)
			}
		} else {
			objectId, err := strconv.ParseUint(lineParts[1], 16, 64)
			if err != nil {
				return err
			}
			object, ok := timeFrameObjectCache[objectId]
			if !ok {
				object = &Object{
					Id:         objectId,
					Properties: make([]*Property, 0),
				}
				timeFrameObjectCache[objectId] = object
				timeFrame.Objects = append(timeFrame.Objects, object)
			}

			err = r.parseObject(object, lineParts[2])
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func (r *reader) readHeader() error {
	foundFileType := false
	foundFileVersion := false

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSuffix(line, "\n")

		matches := keyRe.FindAllStringSubmatch(line, -1)
		if len(matches) != 1 {
			return fmt.Errorf("Failed to parse key pair from line: `%v`", line)
		}

		if matches[0][1] == "FileType" && !foundFileType {
			foundFileType = true
			r.header.FileType = matches[0][1]
		} else if matches[0][1] == "FileVersion" && !foundFileVersion {
			foundFileVersion = true
			r.header.FileVersion = matches[0][2]
		}

		if foundFileType && foundFileVersion {
			break
		}
	}

	r.header.InitialTimeFrame = *NewTimeFrame()
	err := r.readTimeFrame(r.reader, &r.header.InitialTimeFrame, false)
	if err != nil {
		return err
	}

	globalObj := r.header.InitialTimeFrame.Get(0)
	if globalObj == nil {
		return fmt.Errorf("No global object found in initial time frame")
	}

	referenceTimeProperty := globalObj.Get("ReferenceTime")
	if referenceTimeProperty == nil {
		return fmt.Errorf("Global object is missing ReferenceTime")
	}

	referenceTime, err := time.Parse("2006-01-02T15:04:05Z", referenceTimeProperty.Value)
	if err != nil {
		return fmt.Errorf("Failed to parse ReferenceTime: `%v`", referenceTimeProperty.Value)
	}

	r.header.ReferenceTime = referenceTime

	return nil
}

type ObjectPosition struct {
	Longitude float64
	Latitude  float64
	Altitude  float64
	U         float64
	V         float64
	Roll      float64
	Pitch     float64
	Yaw       float64
	Heading   float32
}

func ReadObjectPosition(data string) (ObjectPosition, error) {
	var result ObjectPosition
	parts := strings.Split(data, "|")

	if len(parts) >= 3 {
		if parts[0] != "" {
			lng, err := strconv.ParseFloat(parts[0], 64)
			if err != nil {
				return result, err
			}
			result.Longitude = lng
		}

		if parts[1] != "" {
			lat, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return result, err
			}
			result.Latitude = lat
		}

		if parts[2] != "" {
			alt, err := strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return result, err
			}
			result.Altitude = alt
		}

		if len(parts) == 5 {
			if parts[3] != "" {
				u, err := strconv.ParseFloat(parts[3], 64)
				if err != nil {
					return result, err
				}
				result.U = u
			}
			if parts[4] != "" {
				v, err := strconv.ParseFloat(parts[4], 64)
				if err != nil {
					return result, err
				}
				result.V = v

			}
		}

		if len(parts) == 6 {
			if parts[3] != "" {
				roll, err := strconv.ParseFloat(parts[3], 64)
				if err != nil {
					return result, err
				}
				result.Roll = roll
			}
			if parts[4] != "" {
				pitch, err := strconv.ParseFloat(parts[4], 64)
				if err != nil {
					return result, err
				}
				result.Pitch = pitch
			}
			if parts[5] != "" {
				yaw, err := strconv.ParseFloat(parts[5], 64)
				if err != nil {
					return result, err
				}
				result.Yaw = yaw
			}
		}

		if len(parts) == 9 {
			if parts[3] != "" {
				roll, err := strconv.ParseFloat(parts[3], 64)
				if err != nil {
					return result, err
				}
				result.Roll = roll
			}
			if parts[4] != "" {
				pitch, err := strconv.ParseFloat(parts[4], 64)
				if err != nil {
					return result, err
				}
				result.Pitch = pitch
			}
			if parts[5] != "" {
				yaw, err := strconv.ParseFloat(parts[5], 64)
				if err != nil {
					return result, err
				}
				result.Yaw = yaw
			}
			if parts[6] != "" {
				u, err := strconv.ParseFloat(parts[6], 64)
				if err != nil {
					return result, err
				}
				result.U = u
			}
			if parts[7] != "" {
				v, err := strconv.ParseFloat(parts[7], 64)
				if err != nil {
					return result, err
				}
				result.V = v

			}
			if parts[8] != "" {
				heading, err := strconv.ParseFloat(parts[8], 32)
				if err != nil {
					return result, err
				}
				result.Heading = float32(heading)
			}
		}
	}

	return result, nil
}
