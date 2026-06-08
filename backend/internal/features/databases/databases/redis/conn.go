package redis

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const dialTimeout = 15 * time.Second

// Conn is a minimal RESP client used for connection testing, version/size
// detection, and full-snapshot (SYNC) streaming. It deliberately avoids a
// third-party Redis dependency — the backup path only needs a handful of
// commands plus raw RDB framing.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// DialContext opens a TCP (or TLS) RESP connection. When isTLS is set the
// server certificate is not verified — the same trade-off the MongoDB and
// MySQL connectors make for self-signed internal databases.
func DialContext(ctx context.Context, host string, port int, isTLS bool) (*Conn, error) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	netDialer := &net.Dialer{Timeout: dialTimeout}

	var rawConn net.Conn
	var err error

	if isTLS {
		tlsDialer := &tls.Dialer{
			NetDialer: netDialer,
			Config: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // self-signed internal servers are supported
				ServerName:         host,
			},
		}
		rawConn, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		rawConn, err = netDialer.DialContext(ctx, "tcp", address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", address, err)
	}

	return &Conn{conn: rawConn, reader: bufio.NewReader(rawConn)}, nil
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

// NetConn exposes the underlying connection so callers can wire
// context-driven cancellation by closing it on ctx.Done().
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// Auth sends an AUTH command. Username is optional: when empty the legacy
// password-only form is used; otherwise the Redis 6+ ACL form is sent. An
// empty password skips authentication entirely.
func (c *Conn) Auth(username, password string) error {
	if password == "" {
		return nil
	}

	var err error
	if username != "" {
		err = c.writeCommand("AUTH", username, password)
	} else {
		err = c.writeCommand("AUTH", password)
	}
	if err != nil {
		return fmt.Errorf("failed to send AUTH: %w", err)
	}

	return c.expectStatus()
}

// Ping verifies the connection is live and authenticated.
func (c *Conn) Ping() error {
	if err := c.writeCommand("PING"); err != nil {
		return fmt.Errorf("failed to send PING: %w", err)
	}

	return c.expectStatus()
}

// Info issues INFO and returns the bulk-string payload. An empty section
// returns the default INFO sections.
func (c *Conn) Info(section string) (string, error) {
	args := []string{"INFO"}
	if section != "" {
		args = append(args, section)
	}

	if err := c.writeCommand(args...); err != nil {
		return "", fmt.Errorf("failed to send INFO: %w", err)
	}

	line, err := c.readLine()
	if err != nil {
		return "", err
	}

	if rest, found := strings.CutPrefix(line, "-"); found {
		return "", fmt.Errorf("redis error: %s", rest)
	}

	if !strings.HasPrefix(line, "$") {
		return "", fmt.Errorf("unexpected INFO reply: %s", line)
	}

	length, err := strconv.Atoi(line[1:])
	if err != nil {
		return "", fmt.Errorf("invalid INFO bulk length: %q", line)
	}

	if length < 0 {
		return "", nil
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return "", fmt.Errorf("failed to read INFO payload: %w", err)
	}

	if _, err := c.reader.Discard(2); err != nil {
		return "", fmt.Errorf("failed to read INFO terminator: %w", err)
	}

	return string(payload), nil
}

func (c *Conn) writeCommand(args ...string) error {
	var builder strings.Builder

	builder.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, arg := range args {
		builder.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n")
		builder.WriteString(arg + "\r\n")
	}

	_, err := c.conn.Write([]byte(builder.String()))

	return err
}

func (c *Conn) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}

func (c *Conn) expectStatus() error {
	line, err := c.readLine()
	if err != nil {
		return err
	}

	if rest, found := strings.CutPrefix(line, "-"); found {
		return fmt.Errorf("redis error: %s", rest)
	}

	if !strings.HasPrefix(line, "+") {
		return fmt.Errorf("unexpected redis reply: %s", line)
	}

	return nil
}

// ParseInfoField extracts a single `key:value` field from an INFO reply.
func ParseInfoField(info, key string) string {
	for line := range strings.SplitSeq(info, "\n") {
		line = strings.TrimRight(line, "\r")
		name, value, found := strings.Cut(line, ":")
		if found && name == key {
			return value
		}
	}

	return ""
}
