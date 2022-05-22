package tacview

import (
	"bufio"
	"errors"
	"fmt"
	"hash/crc64"
	"math/bits"
	"net"
	"unicode/utf16"
)

func hashPassword(password string) uint64 {
	password_utf16 := utf16.Encode([]rune(password))

	password_bytes := make([]byte, 2*len(password_utf16))
	for i, r := range password_utf16 {
		password_bytes[2*i+0] = bits.Reverse8(byte(r >> 0))
		password_bytes[2*i+1] = bits.Reverse8(byte(r >> 8))
	}

	hash := crc64.Checksum(password_bytes, crc64.MakeTable(crc64.ECMA))
	return bits.Reverse64(hash)
}

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
		hash := hashPassword(password)

		_, err = conn.Write([]byte(fmt.Sprintf("%x\x00\n", hash)))
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
