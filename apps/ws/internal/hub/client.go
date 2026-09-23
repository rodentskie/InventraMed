package hub

import (
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8 * 1024
	sendBufferSize = 256
)

// client is one WebSocket connection. readPump is its only reader and
// writePump its only writer, as gorilla/websocket requires.
type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan frame
	done chan struct{}
	log  *zap.Logger
}

func newClient(h *Hub, conn *websocket.Conn) *client {
	return &client{
		hub:  h,
		conn: conn,
		send: make(chan frame, sendBufferSize),
		done: make(chan struct{}),
		log:  h.log.With(zap.String("remote_addr", conn.RemoteAddr().String())),
	}
}

// readPump forwards every inbound message to the hub until the connection
// fails, then unregisters the client.
func (c *client) readPump() {
	defer func() {
		c.hub.leave(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.log.Warn("set read deadline failed", zap.Error(err))
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Warn("unexpected websocket close", zap.Error(err))
			}
			return
		}

		c.hub.publish(c, messageType, data)
	}
}

// writePump writes queued frames and keepalive pings. When the hub closes
// send, it writes a close frame and exits.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
		close(c.done)
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
				return
			}
			if !c.write(message.messageType, message.data) {
				return
			}
		case <-ticker.C:
			if !c.write(websocket.PingMessage, nil) {
				return
			}
		}
	}
}

// write sends one frame bounded by writeWait. It returns false when the
// connection is no longer writable.
func (c *client) write(messageType int, data []byte) bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return false
	}

	return c.conn.WriteMessage(messageType, data) == nil
}
