package turnc

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gortc/turn"

	"github.com/pion/logging"

	"github.com/pion/stun"
	"github.com/pion/turnc/internal/testutil"
)

func TestPermission(t *testing.T) {
	t.Run("Refresh", func(t *testing.T) {
		t.Run("Permission", func(t *testing.T) {
			t.Run("Ok", func(t *testing.T) {
				logs := &testutil.Observer{}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:         logs,
					Conn:        connR, // should not be used
					STUN:        stunClient,
					RefreshRate: time.Microsecond,
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				secondReq := make(chan struct{})
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != stun.NewType(stun.MethodCreatePermission, stun.ClassRequest) {
						t.Errorf("bad request type: %s", m.Type)
					}
					reqCount++
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
							stun.Fingerprint,
						),
					})
					if reqCount == 2 {
						secondReq <- struct{}{}
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(allocErr)
				}
				select {
				case <-secondReq:
					// ok
				case <-time.After(time.Second):
					t.Error("timed out")
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				connL.Close()
				testutil.EnsureNoErrors(t, logs)
			})
			t.Run("Error", func(t *testing.T) {
				logs := &testutil.Observer{Level: logging.LogLevelError}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:         logs,
					Conn:        connR, // should not be used
					STUN:        stunClient,
					RefreshRate: time.Microsecond,
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				secondReq := make(chan struct{})
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != stun.NewType(stun.MethodCreatePermission, stun.ClassRequest) {
						t.Errorf("bad request type: %s", m.Type)
					}
					reqCount++
					if reqCount == 2 {
						secondReq <- struct{}{}
					}
					if reqCount > 1 {
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassErrorResponse),
								stun.CodeBadRequest,
								stun.Fingerprint,
							),
						})
					} else {
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								stun.Fingerprint,
							),
						})
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(permErr)
				}
				select {
				case <-secondReq:
					// ok
				case <-time.After(time.Second):
					t.Error("timed out")
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				connL.Close()
				found := false
				for _, l := range logs.All() {
					if l.Level != logging.LogLevelError {
						continue
					}
					if !strings.HasPrefix(l.Message, "failed to refresh permission") {
						t.Errorf("unexpected error message: %s", l.Message)
					}
					found = true
				}
				if !found {
					t.Error("error not logged")
				}
			})
		})
		t.Run("Binding", func(t *testing.T) {
			t.Run("Ok", func(t *testing.T) {
				logs := &testutil.Observer{}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:         logs,
					Conn:        connR, // should not be used
					STUN:        stunClient,
					RefreshRate: time.Microsecond,
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				secondReq := make(chan struct{})
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					switch m.Type {
					case stun.NewType(stun.MethodCreatePermission, stun.ClassRequest):
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								stun.Fingerprint,
							),
						})
					case stun.NewType(stun.MethodChannelBind, stun.ClassRequest):
						reqCount++
						f(stun.Event{
							Message: stun.MustBuild(m,
								stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
							),
						})
						if reqCount == 2 {
							secondReq <- struct{}{}
						}
					default:
						t.Fatalf("unexpected type: %s", m.Type)
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(allocErr)
				}
				if err := p.Bind(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-secondReq:
					// ok
				case <-time.After(time.Second):
					t.Error("timed out")
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				connL.Close()
				testutil.EnsureNoErrors(t, logs)
			})
			t.Run("Error", func(t *testing.T) {
				logs := &testutil.Observer{Level: logging.LogLevelError}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:         logs,
					Conn:        connR, // should not be used
					STUN:        stunClient,
					RefreshRate: time.Microsecond,
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				secondReq := make(chan struct{})
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					switch m.Type {
					case stun.NewType(stun.MethodCreatePermission, stun.ClassRequest):
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								stun.Fingerprint,
							),
						})
					case stun.NewType(stun.MethodChannelBind, stun.ClassRequest):
						reqCount++
						if reqCount == 1 {
							f(stun.Event{
								Message: stun.MustBuild(m,
									stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								),
							})
						} else {
							f(stun.Event{
								Message: stun.MustBuild(m,
									stun.NewType(m.Type.Method, stun.ClassErrorResponse),
									stun.CodeBadRequest,
								),
							})
						}
						if reqCount == 2 {
							secondReq <- struct{}{}
						}
					default:
						t.Fatalf("unexpected type: %s", m.Type)
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(allocErr)
				}
				if err := p.Bind(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-secondReq:
					// ok
				case <-time.After(time.Second):
					t.Error("timed out")
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				connL.Close()
				found := false
				for _, l := range logs.All() {
					if l.Level != logging.LogLevelError {
						continue
					}
					if !strings.Contains(l.Message, "failed to refresh bind") {
						t.Errorf("unexpected error message: %s", l.Message)
					}
					found = true
				}
				if !found {
					t.Error("error not logged")
				}
			})
			t.Run("NoRefresh", func(t *testing.T) {
				logs := &testutil.Observer{}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:             logs,
					Conn:            connR, // should not be used
					STUN:            stunClient,
					RefreshDisabled: true,
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					switch m.Type {
					case stun.NewType(stun.MethodCreatePermission, stun.ClassRequest):
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								stun.Fingerprint,
							),
						})
					case stun.NewType(stun.MethodChannelBind, stun.ClassRequest):
						reqCount++
						f(stun.Event{
							Message: stun.MustBuild(m,
								stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
							),
						})
						if reqCount > 1 {
							t.Error("second request is unexpected")
						}
					default:
						t.Fatalf("unexpected type: %s", m.Type)
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(allocErr)
				}
				if err := p.Bind(); err != nil {
					t.Fatal(err)
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				if err := connL.Close(); err != nil {
					t.Error(err)
				}
				testutil.EnsureNoErrors(t, logs)
			})
			t.Run("Authenticated", func(t *testing.T) {
				logs := &testutil.Observer{}
				connL, connR := net.Pipe()
				stunClient := &testSTUN{}
				c, createErr := New(Options{
					Log:         logs,
					Conn:        connR, // should not be used
					STUN:        stunClient,
					RefreshRate: time.Microsecond,

					Username: "user",
					Password: "secret",
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				integrity := stun.NewLongTermIntegrity("user", "realm", "secret")
				serverNonce := stun.NewNonce("nonce")
				stunClient.indicate = func(m *stun.Message) error {
					t.Fatal("should not be called")
					return nil
				}
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					if m.Type != turn.AllocateRequest {
						t.Errorf("bad request type: %s", m.Type)
					}
					var (
						nonce    stun.Nonce
						username stun.Username
					)
					if parseErr := m.Parse(&nonce, &username); parseErr != nil {
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassErrorResponse),
								stun.NewRealm("realm"),
								serverNonce,
								stun.CodeUnauthorized,
								stun.Fingerprint,
							),
						})
						return nil
					}
					if !bytes.Equal(nonce, serverNonce) {
						t.Error("nonces not equal")
					}
					if integrityErr := integrity.Check(m); integrityErr != nil {
						t.Errorf("integrity check failed: %v", integrityErr)
					}
					f(stun.Event{
						Message: stun.MustBuild(m, stun.NewType(stun.MethodAllocate, stun.ClassSuccessResponse),
							&turn.RelayedAddress{
								Port: 1113,
								IP:   net.IPv4(127, 0, 0, 2),
							},
							stun.Fingerprint,
						),
					})
					return nil
				}
				a, allocErr := c.Allocate()
				if allocErr != nil {
					t.Fatal(allocErr)
				}
				peer := &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: 1001,
				}
				var reqCount int
				secondReq := make(chan struct{})
				stunClient.do = func(m *stun.Message, f func(e stun.Event)) error {
					var (
						nonce    stun.Nonce
						username stun.Username
					)
					if parseErr := m.Parse(&nonce, &username); parseErr != nil {
						t.Error("nonce and username expected")
					}
					if !bytes.Equal(nonce, serverNonce) {
						t.Error("nonces not equal")
					}
					if integrityErr := integrity.Check(m); integrityErr != nil {
						t.Errorf("integrity check failed: %v", integrityErr)
					}
					switch m.Type {
					case stun.NewType(stun.MethodCreatePermission, stun.ClassRequest):
						f(stun.Event{
							Message: stun.MustBuild(m, stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								integrity,
								stun.Fingerprint,
							),
						})
					case stun.NewType(stun.MethodChannelBind, stun.ClassRequest):
						reqCount++
						f(stun.Event{
							Message: stun.MustBuild(m,
								stun.NewType(m.Type.Method, stun.ClassSuccessResponse),
								integrity,
								stun.Fingerprint,
							),
						})
						if reqCount == 2 {
							secondReq <- struct{}{}
						}
					default:
						t.Fatalf("unexpected type: %s", m.Type)
					}
					return nil
				}
				p, permErr := a.CreateUDP(peer)
				if permErr != nil {
					t.Fatal(allocErr)
				}
				if err := p.Bind(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-secondReq:
					// ok
				case <-time.After(time.Second):
					t.Error("timed out")
				}
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				connL.Close()
				testutil.EnsureNoErrors(t, logs)
			})
		})
	})
}
