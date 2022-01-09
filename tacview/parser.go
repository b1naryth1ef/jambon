package tacview

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spkg/bom"
)

type HeaderReader interface {
	ReadHeader() (*Header, error)
}

type RawReader interface {
	HeaderReader

	ReadRawTimeFrame(float64) (*RawTimeFrame, error)
}

type Parser struct {
	r *bufio.Reader
}

func NewParser(reader io.Reader) (*Parser, error) {
	return &Parser{r: bufio.NewReader(bom.NewReader(reader))}, nil
}

func (p *Parser) ReadHeader() (*Header, error) {
	var header Header

	for {
		prefix, err := p.r.Peek(1)
		if err != nil {
			return nil, err
		}

		if unicode.IsDigit(rune(prefix[0])) {
			initialTimeFrame, err := p.ReadTimeFrame(0)
			if err != nil {
				return nil, err
			}
			header.InitialTimeFrame = *initialTimeFrame

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

			header.ReferenceTime = referenceTime
			return &header, nil
		}

		line, err := p.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		lineStr := string(line[:len(line)-1])
		if strings.HasPrefix(lineStr, "FileType=") {
			header.FileType = strings.SplitN(lineStr, "=", 2)[1]
		} else if strings.HasPrefix(lineStr, "FileVersion=") {
			header.FileVersion = strings.SplitN(lineStr, "=", 2)[1]
		} else {
			return nil, fmt.Errorf("Unexpected header line: '%v'", lineStr)
		}
	}

}

var ErrInvalidTimeFrameHeader = errors.New("invalid time frame header")

func (p *Parser) ReadRawTimeFrame(offset float64) (*RawTimeFrame, error) {
	if offset == -1 {
		timeFrameHeaderPrefix, err := p.r.Peek(1)
		if err != nil {
			return nil, err
		}

		if timeFrameHeaderPrefix[0] != '#' {
			return nil, ErrInvalidTimeFrameHeader
		}

		headerLine, err := p.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		offset, err = strconv.ParseFloat(string(headerLine[1:len(headerLine)-1]), 64)
		if err != nil {
			return nil, err
		}
	}

	lines := make([]string, 0)

	var currentLine []byte
	for {
		linePrefix, err := p.r.Peek(1)
		if err != nil {
			return nil, err
		}

		if linePrefix[0] == '#' {
			break
		}

		line, err := p.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		// Escaped line, we have more to read
		if line[len(line)-2] == '\\' {
			currentLine = append(currentLine, line[:len(line)-2]...)
			continue
		}

		if len(currentLine) != 0 {
			lines = append(lines, string(append(currentLine, line[:len(line)-1]...)))
			currentLine = []byte{}
		} else {
			lines = append(lines, string(line[:len(line)-1]))
		}
	}

	return &RawTimeFrame{
		Offset:   offset,
		Contents: lines,
	}, nil
}

func (p *Parser) ReadTimeFrame(offset float64) (*TimeFrame, error) {
	rawTimeFrame, err := p.ReadRawTimeFrame(offset)
	if err != nil {
		return nil, err
	}

	return rawTimeFrame.Parse()
}

func parseObjectLine(line string) (*Object, error) {
	isDelete := false

	if line[0] == '-' {
		line = line[1:]
		isDelete = true
	}

	parts := strings.SplitN(line, ",", 2)

	objectId, err := strconv.ParseUint(parts[0], 16, 64)
	if err != nil {
		return nil, err
	}

	object := &Object{
		Id:         objectId,
		Properties: make([]*Property, 0),
		Deleted:    isDelete,
	}

	if !isDelete {
		var props []string
		if strings.Contains(parts[1], `\,`) {
			var err error
			props, err = splitPropertyTokens(parts[1])
			if err != nil {
				return nil, err
			}
		} else {
			// Fast-er path
			props = strings.Split(parts[1], ",")
		}

		for _, part := range props {
			if len(part) == 0 {
				continue
			}
			partSplit := strings.SplitN(part, "=", 2)
			if len(partSplit) != 2 {
				return nil, fmt.Errorf("Failed to parse property: `%v`", part)
			}

			object.Properties = append(object.Properties, &Property{Key: partSplit[0], Value: partSplit[1]})
		}
	}

	return object, nil
}
