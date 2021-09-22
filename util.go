package jambon

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
			return nil, fmt.Errorf("too many entries in zip file, is it a valid tacview ACMI?")
		}

		return reader.Open(reader.File[0].Name)
	}

	return file, nil
}

// Wraps a zip writer with an unrelated closer
type zipWriteCloser struct {
	io.Writer
	file io.Closer
}

func (z *zipWriteCloser) Close() error {
	return z.file.Close()
}

func openWritableTacView(path string) (io.WriteCloser, error) {
	file, err := os.Create(path)

	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(path, ".zip.acmi") {
		writer := zip.NewWriter(file)

		name := filepath.Base(path)
		name = name[0:len(name)-len(".zip.acmi")] + ".txt.acmi"

		zipFileWriter, err := writer.Create(name)
		if err != nil {
			return nil, err
		}

		return &zipWriteCloser{zipFileWriter, writer}, nil
	}

	return file, nil
}
