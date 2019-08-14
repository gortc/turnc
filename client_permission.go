package turnc

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"gortc.io/turn"
)

// Permission implements net.PacketConn.
type Permission struct {
	log         *zap.Logger
	mux         sync.RWMutex
	ip          net.IP
	client      *Client
	ctx         context.Context
	cancel      func()
	wg          sync.WaitGroup
	refreshRate time.Duration
	conn        []*Connection
}

var (
	// ErrAlreadyBound means that selected permission already has bound channel number.
	ErrAlreadyBound = errors.New("channel already bound")
	// ErrNotBound means that selected permission already has no channel number.
	ErrNotBound = errors.New("channel is not bound")
)

func (p *Permission) refresh() error {
	return p.client.alloc.allocate(turn.PeerAddress{IP: p.ip})
}

func (p *Permission) startLoop(f func()) {
	if p.refreshRate == 0 {
		return
	}
	p.wg.Add(1)
	go func() {
		ticker := time.NewTicker(p.refreshRate)
		defer p.wg.Done()
		for {
			select {
			case <-ticker.C:
				f()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

func (p *Permission) startRefreshLoop() {
	p.startLoop(func() {
		if err := p.refresh(); err != nil {
			p.log.Error("failed to refresh permission", zap.Error(err))
		}
		p.log.Debug("permission refreshed")
	})
}

// WriteTo writes packet b to addr.
func (p *Permission) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	return 0, ErrNotImplemented
}

// Close stops all refreshing loops for permission and removes it from
// allocation.
func (p *Permission) Close() error {
	p.mux.Lock()
	cancel := p.cancel
	p.mux.Unlock()
	cancel()
	p.wg.Wait()
	p.client.alloc.removePermission(p)
	return nil
}

// ErrNotImplemented means that functionality is not currently implemented,
// but it will be (eventually).
var ErrNotImplemented = errors.New("functionality not implemented")

func (p *Permission) removeConn(connection *Connection) {}

// CreateUDP creates new UDP Permission to peer with provided addr.
func (p *Permission) CreateUDP(addr *net.UDPAddr) (*Connection, error) {
	peer := turn.PeerAddress{
		IP:   addr.IP,
		Port: addr.Port,
	}
	c := &Connection{
		log:         p.log,
		peerAddr:    peer,
		client:      p.client,
		refreshRate: p.client.refreshRate,
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.peerL, c.peerR = net.Pipe()
	p.client.mux.Lock()
	p.conn = append(p.conn, c)
	p.client.mux.Unlock()
	return c, nil
}
