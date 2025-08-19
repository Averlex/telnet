# telnet

[![Go version](https://img.shields.io/badge/go-1.24.2+-blue.svg)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/Averlex/telnet.svg)](https://pkg.go.dev/github.com/Averlex/telnet)
[![Go Report Card](https://goreportcard.com/badge/github.com/Averlex/telnet)](https://goreportcard.com/report/github.com/Averlex/telnet)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

CLI telnet tool with support for SIGINT, EOF, and connection timeouts.

## Features

- ✅ Connect to any TCP service
- ✅ `--timeout` flag for connection attempts
- ✅ Graceful shutdown on `SIGINT` (`Ctrl+C`) or `EOF` (`Ctrl+D`)
- ✅ Concurrent bidirectional I/O
- ✅ Clean status output to `stderr`

## Usage

The `--timeout` flag may be omitted - client then uses the default timeout of `10s`

```bash
telnet --timeout=5s <host> <port>
```

Or build and run:

```bash
go build -o go-telnet ./cmd
./go-telnet --timeout=3s localhost 8080
```

### Example

```text
$ ./go-telnet localhost 9000
...Connected to localhost:9000 with timeout 5s
Hello
Server response
...SIGINT received, terminating
...Connection successfully closed
```

## Library Usage

The `telnet` package can be imported and used in other Go programs:

```go
import "github.com/Averlex/telnet/pkg/telnet"

client := telnet.NewClient("localhost:8080", 5*time.Second, os.Stdin, os.Stdout)

if err := client.Connect(); err != nil {
    log.Fatal(err)
}
defer client.Close()

// Run send and receive for a single message.
// Tip: use loops and controllable channel instead for continuous sending and receiving.
go func() { _ = client.Send() }()
go func() { _ = client.Receive() }()

// Use context or channel to control lifecycle.
select {
case <-someDoneChan:
    return
}
```

## Error Handling

The client handles:

- `ErrEOT`: when `EOF` is received from input (e.g. `Ctrl+D`)
- `ErrConnClosed`: when the connection is closed unexpectedly
- Connection timeout via `--timeout`
- All status messages are printed to `stderr`, so output (`stdout`) remains clean for piping.

## Installation

### As a library

```bash
go get github.com/Averlex/telnet/pkg/telnet
```

### As a CLI tool

**Using native Go toolchain**:

```bash
go install github.com/Averlex/telnet/cmd
```

Or **clone and build**:

```bash
git clone https://github.com/Averlex/telnet.git
cd telnet
go build -o go-telnet ./cmd
```

## Testing

In addition to unit tests, an integration test script (`test.sh`) is provided to verify bidirectional communication:

```bash
./test.sh
# ... some test output ...
# Output: PASS
```

The script uses `netcat` (`nc`) as a server and checks that data is correctly exchanged.
