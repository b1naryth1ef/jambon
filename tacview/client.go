package tacview

import (
	"bufio"
	"errors"
	"fmt"
	"hash/crc64"
	"net"
)

/// Creates a new Reader from a TacView Real Time server
func NewRealTimeReader(connStr string, username string, password string) (*Reader, error) {
	conn, err := net.Dial("tcp", connStr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	headerProtocol, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if headerProtocol != "XtraLib.Stream.0\n" {
		return nil, fmt.Errorf("bad header protocol: %v", headerProtocol)
	}

	headerVersion, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if headerVersion != "Tacview.RealTimeTelemetry.0\n" {
		return nil, fmt.Errorf("bad header version %v", headerVersion)
	}

	// Read remote hostname
	_, err = reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	eoh, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if eoh != '\x00' {
		return nil, errors.New("bad or missing end of header")
	}

	_, err = conn.Write([]byte("XtraLib.Stream.0\n"))
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte("Tacview.RealTimeTelemetry.0\n"))
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte(fmt.Sprintf("Client %s\n", username)))
	if err != nil {
		return nil, err
	}

	if password != "" {
		hasher := crc64.New(crc64.MakeTable(crc64.ECMA))
		_, err = hasher.Write([]byte(password))
		if err != nil {
			return nil, err
		}

		_, err = conn.Write([]byte(fmt.Sprintf("%d\x00\n", hasher.Sum64())))
		if err != nil {
			return nil, err
		}
	} else {
		_, err = conn.Write([]byte("\x00\n"))
		if err != nil {
			return nil, err
		}
	}

	return NewReader(reader)
}
