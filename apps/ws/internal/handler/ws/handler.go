package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ConnectionServer takes ownership of an upgraded connection and serves it
// until it closes.
type ConnectionServer interface {
	ServeConn(conn *websocket.Conn)
}

type Handler struct {
	upgrader websocket.Upgrader
	server   ConnectionServer
	log      *zap.Logger
}

func NewHandler(server ConnectionServer, allowedOrigins []string, log *zap.Logger) *Handler {
	return &Handler{
		upgrader: websocket.Upgrader{CheckOrigin: checkOrigin(allowedOrigins)},
		server:   server,
		log:      log,
	}
}

// Serve upgrades the request to a WebSocket and hands it to the server. A
// failed upgrade has already been answered by the upgrader (403 for a
// rejected origin).
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", zap.String("origin", r.Header.Get("Origin")), zap.Error(err))
		return
	}

	h.server.ServeConn(conn)
}

// deviceOrigin is the Origin header the arduinoWebSockets client library
// hardcodes into its handshake. Browsers never send it (a page opened from
// disk sends "null"), so it marks a device rather than a browser.
const deviceOrigin = "file://"

// checkOrigin allows non-browser clients such as the ESP32 (no Origin header,
// or deviceOrigin), and otherwise only origins in allowed, compared
// case-insensitively. A "*" entry allows any origin.
func checkOrigin(allowed []string) func(r *http.Request) bool {
	origins := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		origins[strings.ToLower(origin)] = struct{}{}
	}
	_, allowAll := origins["*"]

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == deviceOrigin || allowAll {
			return true
		}

		_, ok := origins[strings.ToLower(origin)]
		return ok
	}
}
