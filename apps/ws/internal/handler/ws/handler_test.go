package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"apps/ws/internal/hub"
	"apps/ws/internal/wstest"
)

// closingServer closes every connection it is handed, for tests that only
// care whether the upgrade succeeded.
type closingServer struct{}

func (closingServer) ServeConn(conn *websocket.Conn) {
	_ = conn.Close()
}

func serve(t *testing.T, server ConnectionServer, allowedOrigins []string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", NewHandler(server, allowedOrigins, zap.NewNop()).Serve)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return wstest.URL(srv.URL, "/ws")
}

func TestServe_OriginCheck(t *testing.T) {
	allowed := []string{"http://localhost:3000"}
	tests := []struct {
		name    string
		allowed []string
		origin  string
		want    int
	}{
		{name: "no origin header", allowed: allowed, want: http.StatusSwitchingProtocols},
		{name: "arduino device origin", allowed: allowed, origin: "file://", want: http.StatusSwitchingProtocols},
		{name: "null origin is a browser", allowed: allowed, origin: "null", want: http.StatusForbidden},
		{name: "allowed origin", allowed: allowed, origin: "http://localhost:3000", want: http.StatusSwitchingProtocols},
		{name: "allowed origin is case-insensitive", allowed: allowed, origin: "HTTP://LOCALHOST:3000", want: http.StatusSwitchingProtocols},
		{name: "disallowed origin", allowed: allowed, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "empty allowlist rejects browsers", origin: "http://localhost:3000", want: http.StatusForbidden},
		{name: "wildcard allows any origin", allowed: []string{"*"}, origin: "https://any.example", want: http.StatusSwitchingProtocols},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := serve(t, closingServer{}, tt.allowed)
			header := http.Header{}
			if tt.origin != "" {
				header.Set("Origin", tt.origin)
			}

			conn, resp, err := websocket.DefaultDialer.Dial(url, header)
			if conn != nil {
				_ = conn.Close()
			}
			if resp == nil {
				t.Fatalf("dial: no response: %v", err)
			}
			if resp.StatusCode != tt.want {
				t.Errorf("status: got %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestServe_NonWebSocketRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", NewHandler(closingServer{}, nil, zap.NewNop()).Serve)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServe_BroadcastsToOtherClients(t *testing.T) {
	h := hub.New(zap.NewNop())
	go h.Run()
	t.Cleanup(h.Stop)
	url := serve(t, h, nil)
	a, b := wstest.Dial(t, url, nil), wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, a, b)

	if err := a.Conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"red"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := b.Next(t)
	if got.Err != nil || got.Type != websocket.TextMessage || string(got.Data) != `{"status":"red"}` {
		t.Errorf("got type=%d data=%q err=%v, want text {\"status\":\"red\"}", got.Type, got.Data, got.Err)
	}
	a.ExpectNone(t, 100*time.Millisecond)
}
