package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"go.uber.org/zap"

	"gortc.io/turnc"
)

var (
	server = flag.String("server",
		fmt.Sprintf("localhost:3478"),
		"turn server address",
	)
	peer = flag.String("peer",
		"localhost:56780",
		"peer address",
	)
	username = flag.String("u", "user", "username")
	password = flag.String("p", "secret", "password")
)

func main() {
	flag.Parse()
	l, lErr := zap.NewDevelopment()
	if lErr != nil {
		panic(lErr)
	}
	logger := l.Sugar()

	if flag.Arg(0) == "peer" {
		_, port, err := net.SplitHostPort(*peer)
		logger.Info("running in peer mode")
		if err != nil {
			panic(err)
		}
		laddr, err := net.ResolveUDPAddr("udp", ":"+port)
		if err != nil {
			panic(err)
		}
		c, _ := net.ListenUDP("udp", laddr)
		if c == nil {
			panic(c)
		}
		logger.Infof("listening as echo server %s", c.LocalAddr())
		for {
			// Starting echo server.
			buf := make([]byte, 1024)
			n, addr, err := c.ReadFromUDP(buf)
			if err != nil {
				panic(err)
			}
			logger.Infof("got message: [%s] %s", addr, buf[:n])
			// Echoing back.
			if _, err := c.WriteToUDP(buf[:n], addr); err != nil {
				panic(err)
			}
			logger.Infof("echoed back [%s]", addr)
		}
	}
	if *password == "" {
		fmt.Fprintln(os.Stderr, "No password set, auth is required.")
		flag.Usage()
		os.Exit(2)
	}
	// Resolving to TURN server.
	raddr, err := net.ResolveUDPAddr("udp", *server)
	if err != nil {
		panic(err)
	}
	c, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		panic(err)
	}
	logger.Infof("dial server %s -> %s", c.LocalAddr(), c.RemoteAddr())
	client, clientErr := turnc.New(turnc.Options{
		Log:      l,
		Conn:     c,
		Username: *username,
		Password: *password,
	})
	if clientErr != nil {
		panic(clientErr)
	}
	a, allocErr := client.Allocate()
	if allocErr != nil {
		panic(allocErr)
	}
	logger.Info("allocated")
	peerAddr, resolveErr := net.ResolveUDPAddr("udp", *peer)
	if resolveErr != nil {
		panic(resolveErr)
	}
	permission, createErr := a.Create(peerAddr.IP)
	if createErr != nil {
		panic(createErr)
	}
	conn, err := permission.CreateUDP(peerAddr)
	if err != nil {
		panic(err)
	}
	if _, writeRrr := fmt.Fprint(conn, "hello world!"); writeRrr != nil {
		panic(writeRrr)
	}
	buf := make([]byte, 1500)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		panic(readErr)
	}
	logger.Infof("got message: %s", string(buf[:n]))
}
