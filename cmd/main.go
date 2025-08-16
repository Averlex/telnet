// Package main implements a simple CLI for telnet client and handles its workflow.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Averlex/telnet/pkg/telnet"
)

func main() {
	// Flags and args processing.
	var timeout time.Duration
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "telnet [--timeout=5s] <host> <port>")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--timeout=duration] <host> <port>\n", os.Args[0])
		return
	}

	addr := net.JoinHostPort(args[0], args[1])

	// Reading from stdin, printing to stdout.
	client := telnet.NewClient(addr, timeout, os.Stdin, os.Stdout)

	// Connection.
	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "...Connection failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "...Connected to %s with timeout %v\n", addr, max(0, timeout))
	defer func() {
		err := client.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "...Close attempt failed with errors: %s\n", err.Error())
			return
		}
		fmt.Fprintf(os.Stderr, "...Connection successfully closed\n")
	}()

	// Context with signal handling.
	errCh := make(chan error, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Sender goroutine.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := client.Send(); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// Receiving goroutine.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := client.Receive(); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// Errors and signal processing.
	select {
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "...SIGINT received, terminating\n")
	case err := <-errCh:
		switch {
		case errors.Is(err, telnet.ErrEOT):
			fmt.Fprintf(os.Stderr, "...EOF received, terminating\n")
		case errors.Is(err, telnet.ErrConnClosed):
			fmt.Fprintf(os.Stderr, "...Connection is closed, terminating\n")
		default:
			fmt.Fprintf(os.Stderr, "...Unexpected error occurred: %s\n", err.Error())
		}
	}
}
