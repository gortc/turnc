package turnc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"

	"gortc.io/stun"
	"gortc.io/turn"
)

// Allocation reflects TURN Allocation, which is basically an IP:Port on
// TURN server allocated for client.
type Allocation struct {
	log         *zap.Logger
	client      *Client
	relayed     turn.RelayedAddress
	reflexive   stun.XORMappedAddress
	perms       []*Permission // protected with client.mux
	minBound    turn.ChannelNumber
	integrity   stun.MessageIntegrity
	nonce       stun.Nonce
	refreshRate time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

func (a *Allocation) removePermission(p *Permission) {
	a.client.mux.Lock()
	newPerms := make([]*Permission, 0, len(a.perms))
	for _, permission := range a.perms {
		if p == permission {
			continue
		}
		newPerms = append(newPerms, permission)
	}
	a.perms = newPerms
	a.client.mux.Unlock()
}

var errUnauthorised = errors.New("unauthorized")

// allocate expects client.mux locked.
func (c *Client) allocate(req, res *stun.Message) (*Allocation, error) {
	if doErr := c.do(req, res); doErr != nil {
		return nil, doErr
	}
	if res.Type == stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse) {
		var (
			relayed   turn.RelayedAddress
			reflexive stun.XORMappedAddress
			nonce     stun.Nonce
		)
		// Getting relayed and reflexive addresses from response.
		if err := relayed.GetFrom(res); err != nil {
			return nil, err
		}
		if err := reflexive.GetFrom(res); err != nil && err != stun.ErrAttributeNotFound {
			return nil, err
		}
		// Getting nonce from request.
		if err := nonce.GetFrom(req); err != nil && err != stun.ErrAttributeNotFound {
			return nil, err
		}
		a := &Allocation{
			client:      c,
			log:         c.log,
			reflexive:   reflexive,
			relayed:     relayed,
			minBound:    turn.MinChannelNumber,
			integrity:   c.integrity,
			nonce:       nonce,
			refreshRate: c.refreshRate,
		}
		c.alloc = a
		return a, nil
	}
	// Anonymous allocate failed, trying to authenticate.
	if res.Type.Method != stun.MethodAllocate {
		return nil, fmt.Errorf("unexpected response type %s", res.Type)
	}
	var (
		code stun.ErrorCodeAttribute
	)
	if err := code.GetFrom(res); err != nil {
		return nil, err
	}
	if code.Code != stun.CodeUnauthorized {
		return nil, fmt.Errorf("unexpected error code %d", code)
	}
	return nil, errUnauthorised
}

// Allocate creates an allocation for current 5-tuple. Currently there can be
// only one allocation per client, because client wraps one net.Conn.
func (c *Client) Allocate() (*Allocation, error) {
	var (
		nonce stun.Nonce
		res   = stun.New()
	)
	req, reqErr := stun.Build(stun.TransactionID,
		turn.AllocateRequest, turn.RequestedTransportUDP,
		stun.Fingerprint,
	)
	if reqErr != nil {
		return nil, reqErr
	}
	a, allocErr := c.allocate(req, res)
	if allocErr == nil {
		return a, nil
	}
	if allocErr != errUnauthorised {
		return nil, allocErr
	}
	// Anonymous allocate failed, trying to authenticate.
	if err := nonce.GetFrom(res); err != nil {
		return nil, err
	}
	if err := c.realm.GetFrom(res); err != nil {
		return nil, err
	}
	c.realm = append([]byte(nil), c.realm...)
	c.integrity = stun.NewLongTermIntegrity(
		c.username.String(), c.realm.String(), c.password,
	)
	// Trying to authorize.
	if reqErr = req.Build(stun.TransactionID,
		turn.AllocateRequest, turn.RequestedTransportUDP,
		&c.username, &c.realm,
		&nonce,
		&c.integrity, stun.Fingerprint,
	); reqErr != nil {
		return nil, reqErr
	}
	a, err := c.allocate(req, res)
	if err != nil {
		return a, err
	}

	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.startRefreshLoop()

	return a, nil
}

func (a *Allocation) Close() error {
	a.cancel()
	a.client.mux.Lock()
	for _, perm := range a.perms {
		perm.Close()
	}
	a.client.mux.Unlock()

	return nil
}

func (a *Allocation) allocate(peer turn.PeerAddress) error {
	req := stun.New()
	req.TransactionID = stun.NewTransactionID()
	req.Type = stun.NewType(stun.MethodCreatePermission, stun.ClassRequest)
	req.WriteHeader()
	setters := make([]stun.Setter, 0, 10)
	setters = append(setters, &peer)
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
	res := stun.New()
	if doErr := a.client.do(req, res); doErr != nil {
		return doErr
	}
	if res.Type.Class == stun.ClassErrorResponse {
		var code stun.ErrorCodeAttribute
		err := fmt.Errorf("unexpected error response: %s", res.Type)
		if getErr := code.GetFrom(res); getErr == nil {
			err = fmt.Errorf("unexpected error response: %s (error %s)",
				res.Type, code,
			)
		}
		return err
	}

	return nil
}

// Relayed returns the relayed address for the allocation.
func (a *Allocation) Relayed() turn.RelayedAddress {
	return a.relayed
}

// CreateUDP creates new UDP Permission to peer with provided addr.
func (a *Allocation) Create(ip net.IP) (*Permission, error) {
	peer := turn.PeerAddress{
		IP:   ip,
		Port: 0, // Does not matter.
	}
	if err := a.allocate(peer); err != nil {
		return nil, err
	}
	p := &Permission{
		log:         a.log,
		ip:          ip,
		client:      a.client,
		refreshRate: a.client.refreshRate,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.startRefreshLoop()
	a.client.mux.Lock()
	a.perms = append(a.perms, p)
	a.client.mux.Unlock()
	return p, nil
}

func (a *Allocation) startRefreshLoop() {
	a.startLoop(func() {
		if err := a.refresh(); err != nil {
			a.log.Error("failed to refresh permission", zap.Error(err))
		}
		a.log.Debug("permission refreshed")
	})
}

func (a *Allocation) refresh() error {
	res := stun.New()
	req := stun.New()

	err := a.doRefresh(res, req)
	if err != nil {
		return err
	}

	if res.Type == stun.NewType(stun.MethodRefresh, stun.ClassErrorResponse) {
		var errCode stun.ErrorCodeAttribute
		if codeErr := errCode.GetFrom(res); codeErr != nil {
			return codeErr
		}

		if errCode.Code == stun.CodeStaleNonce {
			var nonce stun.Nonce

			if nonceErr := nonce.GetFrom(res); nonceErr != nil {
				return nonceErr
			}
			a.nonce = nonce
			fmt.Println("new nonce", nonce)
			res = stun.New()
			req = stun.New()
			err = a.doRefresh(res, req)
			if err != nil {
				return err
			}
		}
	}

	if res.Type != stun.NewType(stun.MethodChannelBind, stun.ClassSuccessResponse) {
		return fmt.Errorf("unexpected response type %s", res.Type)
	}
	// Success.
	return nil
}

func (a *Allocation) doRefresh(res, req *stun.Message) error {

	req.TransactionID = stun.NewTransactionID()
	req.Type = stun.NewType(stun.MethodRefresh, stun.ClassRequest)
	req.WriteHeader()
	setters := make([]stun.Setter, 0, 10)
	if len(a.integrity) > 0 {
		// Applying auth.
		setters = append(setters,
			a.nonce, a.client.username, a.client.realm, a.integrity, turn.Lifetime{
				Duration: a.refreshRate,
			},
		)
	}
	setters = append(setters, stun.Fingerprint)
	for _, s := range setters {
		if setErr := s.AddTo(req); setErr != nil {
			return setErr
		}
	}
	if doErr := a.client.do(req, res); doErr != nil {
		return doErr
	}

	return nil
}

func (a *Allocation) startLoop(f func()) {
	if a.refreshRate == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(a.refreshRate)
		for {
			select {
			case <-ticker.C:
				f()
			case <-a.ctx.Done():
				return
			}
		}
	}()
}
