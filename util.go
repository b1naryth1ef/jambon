package jambon

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

func openReadableTacView(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(path, ".zip.acmi") {
		stat, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		reader, err := zip.NewReader(file, stat.Size())
		if err != nil {
			return nil, err
		}

		if len(reader.File) != 1 {
			return nil, fmt.Errorf("Too many entries in zip file, is it a valid tacview ACMI?")
		}

		return reader.Open(reader.File[0].Name)
	}

	return file, nil
}
