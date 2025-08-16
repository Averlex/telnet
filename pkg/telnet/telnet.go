// Package telnet provides a telnet client implementation.
//
// The client supports:
//
// - timeout for initial connection;
//
// - EOT signal;
//
// - connection closing.
//
// Main client features are:
//
// - concurrent reading from a given io.ReaderCloser with writing its data to a connection;
//
// - concurrent writing to a given io.Writer with reading its data from a connection.
package telnet

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	// ErrEOT is used in an error chain when EOT signal is received.
	ErrEOT = errors.New("EOT signal received")
	// ErrConnClosed is used in an error chain when the connection to work with is closed.
	ErrConnClosed = errors.New("connection is closed")
)

// Client is an interface for a telnet client.
type Client interface {
	Connect() error
	io.Closer
	Send() error
	Receive() error
}

// Client is used for storing internal client fields.
// It implements Client interface.
type client struct {
	mu      sync.RWMutex
	address string
	timeout time.Duration
	in      io.ReadCloser
	out     io.Writer
	conn    net.Conn
}

// NewClient is a constructor for Client.
// It doesn't perform any validation of the input parameters.
func NewClient(address string, timeout time.Duration, in io.ReadCloser, out io.Writer) Client {
	return &client{
		address: address,
		timeout: timeout,
		in:      in,
		out:     out,
	}
}

// Connect connects to the server with a given timeout.
func (c *client) Connect() error {
	c.mu.RLock()
	address, timeout := c.address, c.timeout
	c.mu.RUnlock() // To avoid blocking while dialing with timeout.
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return nil
}

// Close closes the connection to the server and/or the input stream.
func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	switch {
	case c.conn != nil && c.in != nil:
		inErr := c.in.Close()
		err = c.conn.Close()
		if err == nil {
			err = inErr
		}
	case c.conn != nil:
		err = c.conn.Close()
	case c.in != nil:
		err = c.in.Close()
	default:
		return nil
	}
	c.conn = nil
	return err
}

// Send sends data received from the input stream to the server.
func (c *client) Send() error {
	c.mu.RLock()
	if c.conn == nil || c.in == nil {
		return fmt.Errorf("nil parameter received: connection=%v, input_stream=%v", c.conn == nil, c.in == nil)
	}
	c.mu.RUnlock()

	data, err := c.readOut(c.in)
	if err != nil {
		return err
	}
	// CTRL+D case.
	if len(data) == 0 {
		return ErrEOT
	}

	err = c.writeOut(c.conn, data)
	if err != nil {
		return err
	}

	return nil
}

// Receive reads data from the server and writes it to the output stream.
func (c *client) Receive() error {
	c.mu.RLock()
	if c.conn == nil || c.out == nil {
		return fmt.Errorf("nil parameter received: connection=%v, output_stream=%v", c.conn == nil, c.out == nil)
	}
	c.mu.RUnlock()

	data, err := c.readOut(c.conn)
	if err != nil {
		return err
	}

	err = c.writeOut(c.out, data)
	if err != nil {
		return err
	}

	return nil
}

// readOut is a universal reader which works with io.Reader interface.
func (c *client) readOut(r io.Reader) ([]byte, error) {
	reader := bufio.NewReader(r)
	var res []byte
	for {
		line, err := reader.ReadBytes('\n')
		res = append(res, line...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if reader.Buffered() == 0 {
					return nil, ErrEOT // No more data to read.
				}
				continue // More data in buffer, trying again.
			}
			if errors.Is(err, net.ErrClosed) {
				return nil, fmt.Errorf("%w: %w", ErrConnClosed, err)
			}
			return nil, fmt.Errorf("reading failed: %w", err)
		}
		if reader.Buffered() == 0 {
			break // Not expecting any more data here.
		}
	}
	return res, nil
}

// writeOut is a universal writer which writes data to provided writer.
func (c *client) writeOut(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("%w: %w", ErrConnClosed, err)
		}
		return fmt.Errorf("unable to write out the data: %w", err)
	}
	return nil
}
