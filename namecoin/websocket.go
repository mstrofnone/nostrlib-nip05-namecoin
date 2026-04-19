package namecoin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
)

// wsReadLimit is the maximum size of a single WebSocket message we are
// willing to accept from an ElectrumX server. Verbose transactions
// from `blockchain.transaction.get` can legitimately run to tens of
// kilobytes; 1 MiB is generous but keeps us safely bounded.
const wsReadLimit int64 = 1 << 20

// dialWebSocket opens a WebSocket (plain or TLS) RPC connection to
// the given ElectrumX server. It reuses the same pinned-cert TLS
// config as the TCP+TLS transport so operators who serve a single
// self-signed certificate across both ports are handled identically.
func (c *ElectrumClient) dialWebSocket(ctx context.Context, server ElectrumxServer) (rpcConn, error) {
	transport := effectiveTransport(server)
	scheme := "ws"
	if transport == TransportWSS {
		scheme = "wss"
	}
	address := net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
	url := scheme + "://" + address

	// Bound the dial to our connect timeout, in addition to honouring
	// any deadline on the caller's context.
	dialCtx, cancel := context.WithTimeout(ctx, c.ConnectTimeout)
	defer cancel()

	opts := &websocket.DialOptions{}
	if transport == TransportWSS {
		// Build an http.Transport that carries the same *tls.Config
		// the TCP+TLS path uses. The custom VerifyPeerCertificate is
		// transport-agnostic, so pinned-cert trust continues to work
		// over WebSocket's HTTP upgrade handshake.
		tlsCfg := tlsConfigFor(server)
		opts.HTTPClient = &http.Client{
			Timeout: c.ConnectTimeout + c.ReadTimeout,
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				TLSHandshakeTimeout: c.ConnectTimeout,
			},
		}
	}

	conn, _, err := websocket.Dial(dialCtx, url, opts)
	if err != nil {
		return nil, fmt.Errorf("namecoin: websocket dial %s: %w", url, err)
	}
	conn.SetReadLimit(wsReadLimit)

	return &wsRPCConn{conn: conn}, nil
}

// wsRPCConn is the WebSocket JSON-RPC transport. Unlike the TCP
// variant, WebSocket messages are pre-framed by the protocol layer,
// so requests MUST NOT carry a trailing newline and responses arrive
// as whole JSON messages per frame.
type wsRPCConn struct {
	conn *websocket.Conn
}

func (w *wsRPCConn) writeRequest(ctx context.Context, method string, params []any, id int64) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("namecoin: marshal rpc request: %w", err)
	}
	if err := w.conn.Write(ctx, websocket.MessageText, encoded); err != nil {
		return fmt.Errorf("namecoin: write rpc request: %w", err)
	}
	return nil
}

func (w *wsRPCConn) readResponse(ctx context.Context) (string, error) {
	_, data, err := w.conn.Read(ctx)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *wsRPCConn) close() error {
	return w.conn.Close(websocket.StatusNormalClosure, "")
}

// Compile-time interface checks — catch drift early if the rpcConn
// signature ever evolves.
var (
	_ rpcConn = (*tcpRPCConn)(nil)
	_ rpcConn = (*wsRPCConn)(nil)
)
