package hub

import (
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// frame is one WebSocket message, kept with its original frame type so text
// and binary payloads are relayed unchanged.
type frame struct {
	messageType int
	data        []byte
}

type broadcastMessage struct {
	sender *client
	frame  frame
}

// Hub relays every message from a client to all other connected clients. The
// Run goroutine is the only owner of the clients map, so it needs no mutex.
type Hub struct {
	clients    map[*client]struct{}
	broadcast  chan broadcastMessage
	register   chan *client
	unregister chan *client
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
	log        *zap.Logger
}

func New(log *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		broadcast:  make(chan broadcastMessage),
		register:   make(chan *client),
		unregister: make(chan *client),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		log:        log,
	}
}

// Run processes hub events until Stop is called.
func (h *Hub) Run() {
	defer close(h.done)

	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
			h.log.Debug("client connected", zap.Int("clients", len(h.clients)))
		case c := <-h.unregister:
			h.remove(c)
		case message := <-h.broadcast:
			h.fanOut(message)
		case <-h.stop:
			h.shutdown()
			return
		}
	}
}

// Stop closes every client and blocks until Run has returned. Run must be
// running. Safe to call more than once.
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)
	})
	<-h.done
}

// ServeConn registers conn as a client and pumps messages until the
// connection closes. It blocks, so call it from the connection's goroutine.
func (h *Hub) ServeConn(conn *websocket.Conn) {
	c := newClient(h, conn)
	if !h.join(c) {
		_ = conn.Close()
		return
	}

	go c.writePump()
	c.readPump()
}

// join adds a client. It returns false when the hub has stopped.
func (h *Hub) join(c *client) bool {
	select {
	case h.register <- c:
		return true
	case <-h.stop:
		return false
	}
}

// leave removes a client. It is a no-op once the hub has stopped.
func (h *Hub) leave(c *client) {
	select {
	case h.unregister <- c:
	case <-h.stop:
	}
}

// publish sends a frame to every client except sender. It is a no-op once
// the hub has stopped.
func (h *Hub) publish(sender *client, messageType int, data []byte) {
	message := broadcastMessage{sender: sender, frame: frame{messageType: messageType, data: data}}
	select {
	case h.broadcast <- message:
	case <-h.stop:
	}
}

func (h *Hub) remove(c *client) {
	if _, ok := h.clients[c]; !ok {
		return
	}

	delete(h.clients, c)
	close(c.send)
	h.log.Debug("client disconnected", zap.Int("clients", len(h.clients)))
}

// fanOut never blocks: a client whose send buffer is full is dropped instead
// of stalling everyone else.
func (h *Hub) fanOut(message broadcastMessage) {
	for c := range h.clients {
		if c == message.sender {
			continue
		}

		select {
		case c.send <- message.frame:
		default:
			h.log.Warn("dropping slow client")
			h.remove(c)
		}
	}
}

// shutdown closes every client's send channel, then waits for each write pump
// to send its close frame and exit. Each write is bounded by writeWait.
func (h *Hub) shutdown() {
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		close(c.send)
		delete(h.clients, c)
		clients = append(clients, c)
	}

	for _, c := range clients {
		<-c.done
	}
}
