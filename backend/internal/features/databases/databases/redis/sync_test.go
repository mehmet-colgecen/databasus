package redis

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, header, payload string) ([]byte, error) {
	t.Helper()

	src := bufio.NewReader(strings.NewReader(payload))
	reader, err := newRDBReader(src, header)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
}

func Test_NewRDBReader_WithLengthPrefixedHeader_CopiesExactBytes(t *testing.T) {
	snapshot := "REDIS0011somerdbpayload"

	result, err := readAll(t, "$"+itoa(len(snapshot)), snapshot+"trailing-replication-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != snapshot {
		t.Fatalf("expected %q, got %q", snapshot, string(result))
	}
}

func Test_NewRDBReader_WhenLengthExceedsAvailableBytes_ReturnsUnexpectedEOF(t *testing.T) {
	snapshot := "short"

	_, err := readAll(t, "$100", snapshot)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func Test_NewRDBReader_WithDisklessEOFMarker_StripsMarker(t *testing.T) {
	marker := "0123456789012345678901234567890123456789"
	snapshot := "REDIS0011diskless-payload"

	result, err := readAll(t, "$EOF:"+marker, snapshot+marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != snapshot {
		t.Fatalf("expected %q, got %q", snapshot, string(result))
	}
}

func Test_NewRDBReader_WhenDisklessMarkerSplitAcrossReads_StripsMarker(t *testing.T) {
	marker := "abcdefghijabcdefghijabcdefghijabcdefghij"
	snapshot := strings.Repeat("payload-chunk", 5000)

	src := bufio.NewReaderSize(&slowReader{data: []byte(snapshot + marker), chunk: 7}, 16)
	reader, err := newRDBReader(src, "$EOF:"+marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(result, []byte(snapshot)) {
		t.Fatalf("diskless payload mismatch: got %d bytes, want %d", len(result), len(snapshot))
	}
}

func Test_NewRDBReader_WhenDisklessMarkerMissingAtEnd_ReturnsUnexpectedEOF(t *testing.T) {
	marker := "0123456789012345678901234567890123456789"
	snapshot := "payload-without-trailing-marker"

	_, err := readAll(t, "$EOF:"+marker, snapshot)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func Test_NewRDBReader_WithEmptyEOFMarker_ReturnsError(t *testing.T) {
	_, err := readAll(t, "$EOF:", "anything")
	if err == nil {
		t.Fatal("expected an error for empty EOF marker")
	}
}

func Test_NewRDBReader_WithInvalidLength_ReturnsError(t *testing.T) {
	_, err := readAll(t, "$notanumber", "anything")
	if err == nil {
		t.Fatal("expected an error for invalid bulk length")
	}
}

// slowReader returns at most `chunk` bytes per Read to exercise marker matching
// across buffer boundaries.
type slowReader struct {
	data  []byte
	chunk int
	pos   int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}

	end := min(s.pos+s.chunk, len(s.data))

	n := copy(p, s.data[s.pos:end])
	s.pos += n

	return n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}
