package redis

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const eofMarkerHeaderPrefix = "EOF:"

// StartSync issues SYNC and returns a reader that yields exactly the RDB
// snapshot bytes. It handles both framings Redis may use:
//
//   - length-prefixed: `$<len>\r\n` followed by exactly <len> bytes
//   - diskless EOF:    `$EOF:<40-byte marker>\r\n` followed by the payload and
//     a trailing copy of the marker
//
// The returned reader stops at the end of the snapshot; the live replication
// command stream that a real replica would continue to receive is ignored.
// A short read (connection closed before the full snapshot arrived) surfaces
// as io.ErrUnexpectedEOF so a truncated backup never looks successful.
func (c *Conn) StartSync() (io.Reader, error) {
	if err := c.writeCommand("SYNC"); err != nil {
		return nil, fmt.Errorf("failed to send SYNC: %w", err)
	}

	header, err := c.readSyncHeader()
	if err != nil {
		return nil, err
	}

	return newRDBReader(c.reader, header)
}

// readSyncHeader reads lines until the RDB bulk header arrives. While forking
// to produce the snapshot, Redis may send newline keepalives ("\n") which
// arrive as empty lines and must be skipped.
func (c *Conn) readSyncHeader() (string, error) {
	for {
		line, err := c.readLine()
		if err != nil {
			return "", fmt.Errorf("failed to read SYNC reply: %w", err)
		}

		if line == "" {
			continue
		}

		switch line[0] {
		case '$':
			return line, nil
		case '-':
			return "", fmt.Errorf("redis SYNC error: %s", line[1:])
		default:
			return "", fmt.Errorf("unexpected SYNC reply: %s", line)
		}
	}
}

func newRDBReader(src *bufio.Reader, header string) (io.Reader, error) {
	body := header[1:]

	if marker, found := strings.CutPrefix(body, eofMarkerHeaderPrefix); found {
		markerBytes := []byte(marker)
		if len(markerBytes) == 0 {
			return nil, errors.New("invalid diskless SYNC header: empty EOF marker")
		}

		return &eofReader{src: src, marker: markerBytes}, nil
	}

	length, err := strconv.ParseInt(body, 10, 64)
	if err != nil || length < 0 {
		return nil, fmt.Errorf("invalid SYNC bulk length: %q", header)
	}

	return &lengthReader{src: src, remaining: length}, nil
}

// lengthReader yields exactly `remaining` bytes and reports io.ErrUnexpectedEOF
// if the source ends early.
type lengthReader struct {
	src       io.Reader
	remaining int64
}

func (lr *lengthReader) Read(p []byte) (int, error) {
	if lr.remaining == 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}

	n, err := lr.src.Read(p)
	lr.remaining -= int64(n)

	if errors.Is(err, io.EOF) && lr.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}

	return n, err
}

// eofReader streams a diskless snapshot, withholding the trailing marker. It
// keeps at most len(marker) unemitted bytes buffered so the marker can be
// stripped once the source closes.
type eofReader struct {
	src    *bufio.Reader
	marker []byte
	buf    []byte
	done   bool
}

func (er *eofReader) Read(p []byte) (int, error) {
	if err := er.fill(); err != nil {
		return 0, err
	}

	emittable := len(er.buf)
	if !er.done {
		emittable = len(er.buf) - len(er.marker)
	}

	if emittable <= 0 {
		if er.done {
			return 0, io.EOF
		}

		return 0, nil
	}

	n := copy(p, er.buf[:emittable])
	er.buf = er.buf[n:]

	return n, nil
}

func (er *eofReader) fill() error {
	for !er.done && len(er.buf) <= len(er.marker) {
		chunk := make([]byte, 32*1024)
		n, err := er.src.Read(chunk)
		if n > 0 {
			er.buf = append(er.buf, chunk[:n]...)
		}

		if err == nil {
			continue
		}

		if !errors.Is(err, io.EOF) {
			return err
		}

		if len(er.buf) >= len(er.marker) &&
			bytes.Equal(er.buf[len(er.buf)-len(er.marker):], er.marker) {
			er.buf = er.buf[:len(er.buf)-len(er.marker)]
			er.done = true

			return nil
		}

		return io.ErrUnexpectedEOF
	}

	return nil
}
