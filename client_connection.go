package turnc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"gortc.io/stun"
	"gortc.io/turn"
)

// Connection represents a UDP connectivity between local transport address
// and remote transport address.
type Connection struct {
	log          *zap.Logger
	mux          sync.RWMutex
	number       turn.ChannelNumber
	peerAddr     turn.PeerAddress
	peerL, peerR net.Conn
	client       *Client
	perm         *Permission
	ctx          context.Context
	cancel       func()
	wg           sync.WaitGroup
	refreshRate  time.Duration
}

// Read data from peer.
func (c *Connection) Read(b []byte) (n int, err error) {
	return c.peerR.Read(b)
}

// Bound returns true if channel number is bound for current permission.
func (c *Connection) Bound() bool {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.number.Valid()
}

// Binding returns current channel number or 0 if not bound.
func (c *Connection) Binding() turn.ChannelNumber {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.number
}

func (c *Connection) startLoop(f func()) {
	if c.refreshRate == 0 {
		return
	}
	c.wg.Add(1)
	go func() {
		ticker := time.NewTicker(c.refreshRate)
		defer c.wg.Done()
		for {
			select {
			case <-ticker.C:
				f()
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

// refreshBind performs rebinding of a channel.
func (c *Connection) refreshBind() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.number == 0 {
		return ErrNotBound
	}
	if err := c.bind(c.number); err != nil {
		return err
	}
	c.log.Debug("binding refreshed")
	return nil
}

func (c *Connection) bind(n turn.ChannelNumber) error {
	// Starting transaction.
	a := c.client.alloc
	res := stun.New()
	req := stun.New()
	req.TransactionID = stun.NewTransactionID()
	req.Type = stun.NewType(stun.MethodChannelBind, stun.ClassRequest)
	req.WriteHeader()
	setters := make([]stun.Setter, 0, 10)
	setters = append(setters, &c.peerAddr, n)
	if len(a.integrity) > 0 {
		// Applying auth.
		setters = append(setters,
			a.nonce, a.client.username, a.client.realm, a.integrity,
		)
	}
	setters = append(setters, stun.Fingerprint)
	for _, s := range setters {
		if setErr := s.AddTo(req); setErr != nil {
			return setErr
		}
	}
	if doErr := c.client.do(req, res); doErr != nil {
		return doErr
	}
	if res.Type != stun.NewType(stun.MethodChannelBind, stun.ClassSuccessResponse) {
		return fmt.Errorf("unexpected response type %s", res.Type)
	}
	// Success.
	return nil
}

// Bind performs binding transaction, allocating channel binding for
// the connection.
func (c *Connection) Bind() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.number != 0 {
		return ErrAlreadyBound
	}
	a := c.client.alloc
	a.minBound++
	n := a.minBound
	if err := c.bind(n); err != nil {
		return err
	}
	c.number = n
	c.startLoop(func() {
		if err := c.refreshBind(); err != nil {
			c.log.Error("failed to refresh bind", zap.Error(err))
		}
	})
	return nil
}

// Write sends buffer to peer.
//
// If permission is bound, the ChannelData message will be used.
func (c *Connection) Write(b []byte) (n int, err error) {
	if n := c.Binding(); n.Valid() {
		c.log.Debug("using channel data to write")
		return c.client.sendChan(b, n)
	}
	c.log.Debug("using STUN to write")
	return c.client.sendData(b, &c.peerAddr)
}

// Close stops all refreshing loops for permission and removes it from
// allocation.
func (c *Connection) Close() error {
	cErr := c.peerR.Close()
	c.mux.Lock()
	cancel := c.cancel
	c.mux.Unlock()
	cancel()
	c.wg.Wait()
	c.perm.removeConn(c)
	return cErr
}

// LocalAddr is relayed address from TURN server.
func (c *Connection) LocalAddr() net.Addr {
	return turn.Addr(c.client.alloc.relayed)
}

// RemoteAddr is peer address.
func (c *Connection) RemoteAddr() net.Addr {
	return turn.Addr(c.peerAddr)
}

// SetDeadline implements net.Conn.
func (c *Connection) SetDeadline(t time.Time) error {
	return c.peerR.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.peerR.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return ErrNotImplemented
}
