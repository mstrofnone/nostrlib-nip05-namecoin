package namecoin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeElectrumXWSHandler returns an http.Handler that upgrades incoming
// connections to WebSocket and serves canned JSON-RPC responses for the
// sequence the ElectrumClient issues during NameShow.
//
// The response ordering mirrors the real RPC sequence:
//   1. server.version               → any non-error result
//   2. blockchain.scripthash.get_history → one entry pointing at a fake txid
//   3. blockchain.transaction.get   → one vout with a NAME_UPDATE that
//                                     matches the requested name and a
//                                     value whose "nostr" field is a
//                                     known hex pubkey
//   4. blockchain.headers.subscribe → a height far enough below our
//                                     fake tx height that the name is
//                                     well within its expiry window
//
// This exercises the whole WS dial → read → parse path end-to-end
// without hitting the network.
func fakeElectrumXWSHandler(t *testing.T, identifier, pubkeyHex string) http.Handler {
	// Build the NAME_UPDATE script hex we want the client to parse
	// back out of the "transaction".
	value := `{"nostr":"` + pubkeyHex + `"}`
	script := []byte{opNameUpdate}
	script = append(script, pushData([]byte(identifier))...)
	script = append(script, pushData([]byte(value))...)
	script = append(script, op2Drop, opDrop)
	// Stub P2PKH-ish tail so the parser's "we don't care what follows
	// the DROPs" assumption holds.
	script = append(script, 0x76, 0xa9, 0x14)
	scriptHex := hexString(script)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("ws accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusInternalError, "test server shutting down")

		ctx := r.Context()

		respond := func(id any, result any) {
			payload := map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  result,
			}
			raw, _ := json.Marshal(payload)
			if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
				t.Logf("ws write: %v", err)
			}
		}

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var req struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				t.Logf("ws request parse: %v", err)
				return
			}
			switch req.Method {
			case "server.version":
				respond(req.ID, []string{"ElectrumX 1.16.0", electrumProtocolVersion})
			case "blockchain.scripthash.get_history":
				respond(req.ID, []map[string]any{
					{"tx_hash": "deadbeefcafebabe", "height": 600000},
				})
			case "blockchain.transaction.get":
				respond(req.ID, map[string]any{
					"txid": "deadbeefcafebabe",
					"vout": []map[string]any{
						{"scriptPubKey": map[string]any{"hex": scriptHex}},
					},
				})
			case "blockchain.headers.subscribe":
				respond(req.ID, map[string]any{"height": 600010})
			default:
				respond(req.ID, nil)
			}
		}
	})
}

func hexString(b []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, v := range b {
		out = append(out, hexDigits[v>>4], hexDigits[v&0x0f])
	}
	return string(out)
}

func TestWebSocketTransport_NameShow(t *testing.T) {
	identifier := "d/wsfake"
	pubkey := "aaaa000000000000000000000000000000000000000000000000000000000001"

	ts := httptest.NewServer(fakeElectrumXWSHandler(t, identifier, pubkey))
	defer ts.Close()

	// Parse the httptest URL so we can point the client at it as
	// host+port with the plain-WS transport. httptest.Server is HTTP,
	// so TransportWS (not WSS) is what we want here — it verifies the
	// framing/routing independently of the TLS plumbing.
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	srv := ElectrumxServer{
		Host:      u.Hostname(),
		Port:      port,
		Transport: TransportWS,
	}

	client := NewElectrumClient()
	client.ConnectTimeout = 3 * time.Second
	client.ReadTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := client.NameShow(ctx, identifier, srv)
	if err != nil {
		t.Fatalf("NameShow over WS: %v", err)
	}
	if result == nil {
		t.Fatal("NameShow returned nil result")
	}
	if result.Name != identifier {
		t.Errorf("name = %q, want %q", result.Name, identifier)
	}
	if !strings.Contains(result.Value, pubkey) {
		t.Errorf("value = %q, want to contain %q", result.Value, pubkey)
	}
}

func TestEffectiveTransport(t *testing.T) {
	cases := []struct {
		in   ElectrumxServer
		want Transport
	}{
		// v0.1.x shape: UseSSL=true, Transport unset → TCPTLS.
		{ElectrumxServer{UseSSL: true}, TransportTCPTLS},
		// v0.1.x shape: UseSSL=false, Transport unset → plain TCP.
		{ElectrumxServer{UseSSL: false}, TransportTCP},
		// Explicit Transport wins over UseSSL when non-zero.
		{ElectrumxServer{UseSSL: true, Transport: TransportWSS}, TransportWSS},
		{ElectrumxServer{UseSSL: false, Transport: TransportWS}, TransportWS},
		{ElectrumxServer{UseSSL: false, Transport: TransportTCPTLS}, TransportTCP}, // TCPTLS is zero value; falls back to UseSSL
	}
	for i, tc := range cases {
		if got := effectiveTransport(tc.in); got != tc.want {
			t.Errorf("case %d: effectiveTransport(%+v) = %d, want %d", i, tc.in, got, tc.want)
		}
	}
}

func TestDefaultElectrumXServersWSS(t *testing.T) {
	if len(DefaultElectrumXServersWSS) == 0 {
		t.Fatal("DefaultElectrumXServersWSS is empty")
	}
	for _, srv := range DefaultElectrumXServersWSS {
		if srv.Transport != TransportWSS {
			t.Errorf("%s:%d has Transport=%d, want WSS", srv.Host, srv.Port, srv.Transport)
		}
		if !srv.UsePinnedTrustStore {
			t.Errorf("%s:%d: WSS default servers must pin the trust store", srv.Host, srv.Port)
		}
	}
}
