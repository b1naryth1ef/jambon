package tacview

import (
	"io"
	"log"
	"time"
)

func TrimRaw(reader RawReader, writer RawWriter, start, end float64) error {
	header, err := reader.ReadHeader()
	if err != nil {
		return err
	}
	log.Printf("Header %v", header)

	aliveObjects := make(map[uint64]*Object)

	for {
		rawTimeFrame, err := reader.ReadRawTimeFrame(-1)
		if err != nil {
			return err
		}

		if rawTimeFrame.Offset >= start {
			break
		}
		parsed, err := rawTimeFrame.Parse()
		if err != nil {
			return err
		}

		for _, object := range parsed.Objects {
			existingObject := aliveObjects[object.Id]
			if object.Deleted && existingObject != nil {
				delete(aliveObjects, object.Id)
			} else if !object.Deleted {
				if existingObject == nil {
					aliveObjects[object.Id] = object
				} else {
					for _, newProp := range object.Properties {
						existingObject.Set(newProp.Key, newProp.Value)
					}
				}
			}
		}
	}

	log.Printf("%v pre-start objects", len(aliveObjects))
	referenceTime := header.ReferenceTime.Add(time.Second * time.Duration(start))

	// We copy the initial time frame completely
	initialTimeFrame := NewTimeFrame()
	initialTimeFrame.Offset = 0
	initialTimeFrame.Objects = header.InitialTimeFrame.Objects
	for _, object := range aliveObjects {
		initialTimeFrame.Objects = append(initialTimeFrame.Objects, object)
	}

	err = writer.WriteHeader(&Header{
		FileType:         header.FileType,
		FileVersion:      header.FileVersion,
		ReferenceTime:    referenceTime,
		InitialTimeFrame: *initialTimeFrame,
	})
	if err != nil {
		return err
	}

	for {
		rawTimeFrame, err := reader.ReadRawTimeFrame(-1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if rawTimeFrame.Offset > end {
			break
		}

		rawTimeFrame.Offset = rawTimeFrame.Offset - start
		writer.Write(rawTimeFrame)
	}

	return nil
}
