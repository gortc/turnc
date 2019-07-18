package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/gortc/turn"
	"github.com/gortc/turnc"

	"github.com/pion/logging"
)

const (
	udp      = "udp"
	peerPort = 56780
)

// resolve tries to resolve provided address multiple times, waiting
// between attempts and calling panic if it fails after 10 attempts.
func resolve(host string, port int) *net.UDPAddr {
	addr := fmt.Sprintf("%s:%d", host, port)
	var (
		resolved   *net.UDPAddr
		resolveErr error
	)
	for i := 0; i < 10; i++ {
		resolved, resolveErr = net.ResolveUDPAddr(udp, addr)
		if resolveErr == nil {
			return resolved
		}
		time.Sleep(time.Millisecond * 300 * time.Duration(i))
	}
	panic(resolveErr)
}

func runPeer(logger logging.LeveledLogger) {
	laddr, err := net.ResolveUDPAddr(udp, fmt.Sprintf(":%d", peerPort))
	if err != nil {
		panic(fmt.Sprintf("failed to resolve UDP addr: %v", err))
	}
	c, err := net.ListenUDP(udp, laddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	logger.Infof("listening as echo server at %d", c.LocalAddr())
	for {
		// Starting echo server.
		buf := make([]byte, 1024)
		n, addr, err := c.ReadFromUDP(buf)
		if err != nil {
			panic(fmt.Sprintf("failed to read: %v", err))
		}
		logger.Infof("got message: body %s; raddr: %s", string(buf[:n]), addr)
		// Echoing back.
		if _, err := c.WriteToUDP(buf[:n], addr); err != nil {
			panic(fmt.Sprintf("failed to write back: %v", err))
		}
		logger.Infof("echoed back to %d", addr)
	}
}

func main() {
	flag.Parse()
	lf := logging.NewDefaultLoggerFactory()
	logger := lf.NewLogger("test")
	if flag.Arg(0) == "peer" {
		runPeer(logger)
	}
	// Resolving server and peer addresses.
	var (
		serverAddr = resolve("turn-server", turn.DefaultPort)
		echoAddr   = resolve("turn-peer", peerPort)
	)
	// Creating connection from client to server.
	c, err := net.DialUDP(udp, nil, serverAddr)
	if err != nil {
		logger.Errorf("failed to dial to TURN server: %v", err)
		os.Exit(2)
	}
	logger.Infof("dialed server: laddr=%s raddr=%s peer=%s", c.LocalAddr(), c.RemoteAddr(), echoAddr)
	client, err := turnc.New(turnc.Options{
		Log:      logger,
		Conn:     c,
		Username: "user",
		Password: "secret",
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %v", err))
	}
	a, err := client.Allocate()
	if err != nil {
		panic(fmt.Sprintf("failed to create allocation: %v", err))
	}
	p, err := a.Create(echoAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to create permission: %v", err))
	}
	// Sending and receiving "hello" message.
	if _, err := fmt.Fprint(p, "hello"); err != nil {
		panic(fmt.Sprintf("failed to write data"))
	}
	sent := []byte("hello")
	got := make([]byte, len(sent))
	if _, err = p.Read(got); err != nil {
		panic(fmt.Sprintf("failed to read data: %v", err))
	}
	if !bytes.Equal(got, sent) {
		panic(fmt.Sprintf("got incorrect data"))
	}
	// Repeating via channel binding.
	for i := range got {
		got[i] = 0
	}
	if bindErr := p.Bind(); bindErr != nil {
		panic(fmt.Sprintf("failed to bind: %v", err))
	}
	if !p.Bound() {
		panic(fmt.Sprintf("should be bound"))
	}
	logger.Infof("bound to channel 0x%x", int(p.Binding()))
	// Sending and receiving "hello" message.
	if _, err := fmt.Fprint(p, "hello"); err != nil {
		panic(fmt.Sprintf("failed to write data"))
	}
	if _, err = p.Read(got); err != nil {
		panic(fmt.Sprintf("failed to read data: %v", err))
	}
	if !bytes.Equal(got, sent) {
		panic(fmt.Sprintf("got incorrect data"))
	}
	logger.Info("closing")
}
