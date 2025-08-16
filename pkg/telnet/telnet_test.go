package telnet

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:")
		require.NoError(t, err)
		defer func() { require.NoError(t, l.Close()) }()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()

			in := &bytes.Buffer{}
			out := &bytes.Buffer{}

			timeout, err := time.ParseDuration("10s")
			require.NoError(t, err)

			client := NewClient(l.Addr().String(), timeout, io.NopCloser(in), out)
			require.NoError(t, client.Connect())
			defer func() { require.NoError(t, client.Close()) }()

			in.WriteString("hello\n")
			err = client.Send()
			require.NoError(t, err)

			err = client.Receive()
			require.NoError(t, err)
			require.Equal(t, "world\n", out.String())
		}()

		go func() {
			defer wg.Done()

			conn, err := l.Accept()
			require.NoError(t, err)
			require.NotNil(t, conn)
			defer func() { require.NoError(t, conn.Close()) }()

			request := make([]byte, 1024)
			n, err := conn.Read(request)
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(request)[:n])

			n, err = conn.Write([]byte("world\n"))
			require.NoError(t, err)
			require.NotEqual(t, 0, n)
		}()

		wg.Wait()
	})
}

func TestClient_Send(t *testing.T) {
	testCases := []struct {
		name          string
		inputData     string
		shouldConnect bool
		err           error
		expectedData  string
	}{
		{
			name:          "single line",
			inputData:     "hello\n",
			shouldConnect: true,
			err:           nil,
			expectedData:  "hello\n",
		},
		{
			name:          "multiple lines",
			inputData:     "first\nsecond\nthird\n",
			shouldConnect: true,
			err:           nil,
			expectedData:  "first\nsecond\nthird\n",
		},
		{
			name:          "empty line",
			inputData:     "\n",
			shouldConnect: true,
			err:           nil,
			expectedData:  "\n",
		},
		{
			name:          "multiple empty lines",
			inputData:     "\n\n\n",
			shouldConnect: true,
			err:           nil,
			expectedData:  "\n\n\n",
		},
		{
			name:          "closed input",
			inputData:     "",
			shouldConnect: true,
			err:           ErrEOT,
			expectedData:  "",
		},
		{
			name:          "nil connection",
			inputData:     "data\n",
			shouldConnect: false,
			err:           nil,
			expectedData:  "",
		},
		{
			name:          "nil input",
			inputData:     "",
			shouldConnect: true,
			err:           nil,
			expectedData:  "",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			if tC.name == "nil connection" {
				t.Log("")
			}
			var (
				listener net.Listener
				client   Client
				err      error
			)

			if tC.shouldConnect && tC.name != "nil connection" {
				listener, err = net.Listen("tcp", "127.0.0.1:")
				require.NoError(t, err, "failed to create listener")
				t.Cleanup(func() { require.NoError(t, listener.Close()) })

				go func() {
					conn, err := listener.Accept()
					require.NoError(t, err, "server failed to accept connection")
					defer conn.Close()

					if tC.expectedData == "" {
						return
					}

					buf := make([]byte, 1024)
					n, err := conn.Read(buf)
					require.NoError(t, err, "server failed to read data")
					require.Equal(t, tC.expectedData, string(buf[:n]), "server received unexpected data")
				}()
			}

			// Preparing input stream.
			var in io.ReadCloser
			if tC.name == "nil input" {
				in = nil
			} else {
				in = io.NopCloser(bytes.NewBufferString(tC.inputData))
			}

			// Creating client.
			if tC.name == "nil connection" {
				client = NewClient("localhost:0", 1*time.Millisecond, in, nil)
			} else {
				client = NewClient(listener.Addr().String(), 10*time.Second, in, nil)
				if tC.shouldConnect {
					require.NoError(t, client.Connect(), "connection failed, but shouldn't")
				}
			}

			err = client.Send()
			switch {
			case tC.err != nil:
				require.ErrorIs(t, err, tC.err, "unexpected error received: %w", err)
			case tC.name == "nil connection" || tC.name == "nil input":
				require.Error(t, err, "expected error on Send(), got nil")
			default:
				require.NoError(t, err, "no error expected")
			}
		})
	}
}

func TestClient_Receive(t *testing.T) {
	testCases := []struct {
		name           string
		serverData     string
		shouldConnect  bool
		err            error
		expectedOutput string
	}{
		{
			name:           "single line",
			serverData:     "hello\n",
			shouldConnect:  true,
			err:            nil,
			expectedOutput: "hello\n",
		},
		{
			name:           "multiple lines",
			serverData:     "first\nsecond\nthird\n",
			shouldConnect:  true,
			err:            nil,
			expectedOutput: "first\nsecond\nthird\n",
		},
		{
			name:           "empty line",
			serverData:     "\n",
			shouldConnect:  true,
			err:            nil,
			expectedOutput: "\n",
		},
		{
			name:           "multiple empty lines",
			serverData:     "\n\n\n",
			shouldConnect:  true,
			err:            nil,
			expectedOutput: "\n\n\n",
		},
		{
			name:           "closed connection",
			serverData:     "",
			shouldConnect:  true,
			err:            ErrEOT,
			expectedOutput: "",
		},
		{
			name:           "nil connection",
			serverData:     "data\n",
			shouldConnect:  false,
			err:            nil,
			expectedOutput: "",
		},
		{
			name:           "nil output",
			serverData:     "data\n",
			shouldConnect:  true,
			err:            nil,
			expectedOutput: "",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			var (
				listener net.Listener
				client   Client
				err      error
			)

			buf := &bytes.Buffer{}

			if tC.shouldConnect && tC.name != "nil connection" {
				listener, err = net.Listen("tcp", "127.0.0.1:")
				require.NoError(t, err, "failed to create listener")
				t.Cleanup(func() { require.NoError(t, listener.Close()) })

				go func() {
					conn, err := listener.Accept()
					require.NoError(t, err, "server failed to accept connection")
					defer conn.Close()

					if tC.serverData != "" {
						_, err = conn.Write([]byte(tC.serverData))
						require.NoError(t, err, "server failed to write data")
					}
				}()
			}

			// Prepare output stream.
			var out io.Writer
			if tC.name == "nil output" {
				out = nil
			} else {
				out = buf
			}

			// Create client.
			if tC.name == "nil connection" {
				client = NewClient("localhost:0", 1*time.Millisecond, nil, out)
			} else {
				client = NewClient(listener.Addr().String(), 10*time.Second, nil, out)
				if tC.shouldConnect {
					require.NoError(t, client.Connect(), "connection failed, but shouldn't")
				}
			}

			err = client.Receive()

			switch {
			case tC.err != nil:
				require.ErrorIs(t, err, tC.err, "unexpected error received: %w", err)
			case tC.name == "nil connection" || tC.name == "nil output":
				require.Error(t, err, "expected error on Receive(), got nil")
			default:
				require.NoError(t, err, "no error expected")
			}

			require.Equal(t, tC.expectedOutput, buf.String(), "output mismatch")
		})
	}
}

func TestClient_Connect(t *testing.T) {
	testCases := []struct {
		name      string
		address   string
		timeout   time.Duration
		expectErr bool
	}{
		{
			name:      "successful connection",
			address:   "localhost:0",
			timeout:   10 * time.Second,
			expectErr: false,
		},
		{
			name:      "invalid address",
			address:   "localhost:0",
			timeout:   10 * time.Second,
			expectErr: true,
		},
		{
			name:      "zero timeout",
			address:   "localhost:0",
			timeout:   0,
			expectErr: true,
		},
		{
			name:      "negative timeout",
			address:   "localhost:0",
			timeout:   (-10) * time.Second,
			expectErr: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			var listener net.Listener
			var err error

			if tC.name == "successful connection" {
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err, "failed to create listener")
				t.Cleanup(func() { require.NoError(t, listener.Close()) })
				tC.address = listener.Addr().String()
			} else {
				// Use unreachable port.
				tC.address = "localhost:1"
			}

			client := NewClient(tC.address, tC.timeout, nil, nil)
			err = client.Connect()

			if tC.expectErr {
				require.Error(t, err, "expected error, got nil")
			} else {
				require.NoError(t, err, "no error expected, but got one")
			}
		})
	}
}

func TestClient_Close(t *testing.T) {
	testCases := []struct {
		name         string
		connected    bool
		withInput    bool
		closeConn    bool
		expectClosed bool
	}{
		{
			name:         "conn and input",
			connected:    true,
			withInput:    true,
			closeConn:    true,
			expectClosed: true,
		},
		{
			name:         "input only",
			connected:    false,
			withInput:    true,
			closeConn:    false,
			expectClosed: false,
		},
		{
			name:         "conn only",
			connected:    true,
			withInput:    false,
			closeConn:    true,
			expectClosed: true,
		},
		{
			name:         "conn closed",
			connected:    true,
			withInput:    false,
			closeConn:    true,
			expectClosed: true,
		},
		{
			name:         "nil conn and input",
			connected:    false,
			withInput:    false,
			closeConn:    false,
			expectClosed: false,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			var listener net.Listener
			var err error

			if tC.connected {
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err, "failed to create listener")
				t.Cleanup(func() { require.NoError(t, listener.Close()) })

				client := NewClient(listener.Addr().String(), 10*time.Second, nil, nil)
				require.NoError(t, client.Connect(), "Connect() failed")

				if tC.withInput {
					in := io.NopCloser(bytes.NewBuffer(nil))
					clientWithIn := NewClient(listener.Addr().String(), 10*time.Second, in, nil)
					require.NoError(t, clientWithIn.Connect(), "Connect() with input failed")
					err = clientWithIn.Close()
				} else {
					err = client.Close()
				}
			} else {
				client := NewClient("localhost:0", 1*time.Second, nil, nil)
				err = client.Close()
			}

			require.NoError(t, err, "Close() should not return error")
		})
	}
}
