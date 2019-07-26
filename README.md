[![Build Status](https://travis-ci.com/gortc/turnc.svg?branch=master)](https://travis-ci.com/gortc/turnc)
[![codecov](https://codecov.io/gh/gortc/turnc/branch/master/graph/badge.svg)](https://codecov.io/gh/gortc/turnc)
[![GoDoc](https://godoc.org/github.com/gortc/turnc?status.svg)](https://godoc.org/github.com/gortc/turnc)

# TURNc

Package `turnc` implements TURN [[RFC5766](https://tools.ietf.org/html/rfc5766)] client.
Based on [gortc/stun](https://github.com/gortc/stun) and [gortc/turn](https://github.com/gortc/turn) packages.
See [gortcd](https://github.com/gortc/gortcd) for TURN server.

## Example
If we have a TURN Server listening on example.com port 3478 (UDP) and
know correct credentials, we can use it to relay data to peer which
is listens on 10.0.0.1:34587 (UDP) and writing back any data it receives:
```go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"gortc.io/turnc"
)

func main() {
	// Resolving to TURN server.
	raddr, err := net.ResolveUDPAddr("udp", "example.com:3478")
	if err != nil {
		panic(err)
	}
	c, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		panic(err)
	}
	client, clientErr := turnc.New(turnc.Options{
		Conn:     c,
		// Credentials:
		Username: "user",
		Password: "secret",
	})
	if clientErr != nil {
		panic(clientErr)
	}
	a, allocErr := client.Allocate()
	if allocErr != nil {
		panic(allocErr)
	}
	peerAddr, resolveErr := net.ResolveUDPAddr("udp", "10.0.0.1:34587")
	if resolveErr != nil {
		panic(resolveErr)
	}
	permission, createErr := a.Create(peerAddr)
	if createErr != nil {
		panic(createErr)
	}
	// Permission implements net.Conn.
	if _, writeRrr := fmt.Fprint(permission, "hello world!"); writeRrr != nil {
		panic(peerAddr)
	}
	buf := make([]byte, 1500)
	n, readErr := permission.Read(buf)
	if readErr != nil {
		panic(readErr)
	}
	fmt.Println("got message:", string(buf[:n]))
	// Also you can use ChannelData messages to reduce overhead:
	if err := permission.Bind(); err != nil {
		panic(err)
	}
}
```
### Server for experiments
You can use the `turn.gortc.io:3478` *gortcd* TURN server instance for experiments.
The only allowed peer address is `127.0.0.1:56780` (that is running near the *gortcd*)
which will echo back any message it receives. Username is `user`, password is `secret`.

So just change `example.com:3478` to `turn.gortc.io:3478` and `10.0.0.1:34587` to `127.0.0.1:56780`
in previous example and it should just work:
```bash
$ go get gortc.io/turnc/cmd/turn-client
$ turn-client -server turn.gortc.io:3478 -peer 127.0.0.1:56780
0045	INFO	dial server	{"laddr": "192.168.88.10:36893", "raddr": "159.69.47.227:3478"}
0094	DEBUG	multiplexer	read	{"n": 104}
0095	DEBUG	multiplexer	got STUN data
0095	DEBUG	multiplexer	stun message	{"msg": "allocate error response l=84 attrs=5 id=PcPWfgQhiNnc7HR9"}
0144	DEBUG	multiplexer	read	{"n": 140}
0144	DEBUG	multiplexer	got STUN data
0144	DEBUG	multiplexer	stun message	{"msg": "allocate success response l=120 attrs=8 id=HNMg9zYhvO3D4wp8"}
0144	INFO	allocated
0192	DEBUG	multiplexer	read	{"n": 116}
0192	DEBUG	multiplexer	got STUN data
0192	DEBUG	multiplexer	stun message	{"msg": "create permission success response l=96 attrs=6 id=NVfoJXcKV8VaHpvK"}
0193	DEBUG	allocation.permission	using STUN to write
0242	DEBUG	multiplexer	read	{"n": 56}
0242	DEBUG	multiplexer	got STUN data
0242	DEBUG	multiplexer	stun message	{"msg": "data indication l=36 attrs=3 id=RoZvzIOY3/NG9GkT"}
0242	INFO	got message	{"body": "hello world!"}
```

*Work in progress*

## License

BSD
