package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"apps/ws/internal/wstest"
)

const testTimeout = wstest.Timeout

func startHub(t *testing.T) *Hub {
	t.Helper()

	h := New(zap.NewNop())
	go h.Run()
	t.Cleanup(h.Stop)

	return h
}

// newTestClient returns a client with no connection. done is already closed
// because there is no write pump for Stop to wait on.
func newTestClient(buffer int) *client {
	done := make(chan struct{})
	close(done)

	return &client{send: make(chan frame, buffer), done: done}
}

func receive(t *testing.T, c *client) (frame, bool) {
	t.Helper()

	select {
	case f, ok := <-c.send:
		return f, ok
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting on client send")
		return frame{}, false
	}
}

func TestPublish_ReachesOthersNotSender(t *testing.T) {
	h := startHub(t)
	sender, a, b := newTestClient(1), newTestClient(1), newTestClient(1)
	for _, c := range []*client{sender, a, b} {
		if !h.join(c) {
			t.Fatal("join: hub unexpectedly stopped")
		}
	}

	h.publish(sender, websocket.BinaryMessage, []byte("led"))

	for _, c := range []*client{a, b} {
		f, ok := receive(t, c)
		if !ok || f.messageType != websocket.BinaryMessage || string(f.data) != "led" {
			t.Errorf("got %+v (open=%v), want binary \"led\"", f, ok)
		}
	}
	// Run handles events in order, so once this join returns the publish has
	// been fully fanned out.
	h.join(newTestClient(0))
	if len(sender.send) != 0 {
		t.Error("sender received its own message")
	}
}

func TestPublish_DropsSlowClient(t *testing.T) {
	h := startHub(t)
	sender, slow := newTestClient(0), newTestClient(1)
	slow.send <- frame{}
	h.join(sender)
	h.join(slow)

	h.publish(sender, websocket.TextMessage, []byte("x"))

	receive(t, slow)
	if _, ok := receive(t, slow); ok {
		t.Fatal("slow client send still open, want closed")
	}
	// Leaving after being dropped must not close send a second time.
	h.leave(slow)
	h.leave(slow)
}

func TestLeave_ClosesSend(t *testing.T) {
	h := startHub(t)
	c := newTestClient(0)
	h.join(c)

	h.leave(c)

	if _, ok := receive(t, c); ok {
		t.Fatal("send still open after leave")
	}
}

func TestStop_ClosesClientsAndUnblocksCalls(t *testing.T) {
	h := New(zap.NewNop())
	go h.Run()
	a, b := newTestClient(0), newTestClient(0)
	h.join(a)
	h.join(b)

	h.Stop()
	h.Stop()

	for _, c := range []*client{a, b} {
		if _, ok := receive(t, c); ok {
			t.Error("send still open after Stop")
		}
	}

	done := make(chan struct{})
	go func() {
		if h.join(newTestClient(0)) {
			t.Error("join after Stop: got true, want false")
		}
		h.leave(a)
		h.publish(a, websocket.TextMessage, []byte("x"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("hub calls blocked after Stop")
	}
}

func serveHub(t *testing.T, h *Hub) string {
	t.Helper()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		h.ServeConn(conn)
	}))
	t.Cleanup(srv.Close)

	return wstest.URL(srv.URL, "")
}

func TestServeConn_AfterStopClosesConnection(t *testing.T) {
	h := New(zap.NewNop())
	go h.Run()
	h.Stop()
	c := wstest.Dial(t, serveHub(t, h), nil)

	if msg := c.Next(t); msg.Err == nil {
		t.Fatalf("got message %q, want closed connection", msg.Data)
	}
}

func TestServeConn_RelaysFrameTypes(t *testing.T) {
	h := startHub(t)
	url := serveHub(t, h)
	a, b := wstest.Dial(t, url, nil), wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, a, b)

	sent := []wstest.Message{
		{Type: websocket.TextMessage, Data: []byte("green")},
		{Type: websocket.BinaryMessage, Data: []byte{0x00, 0xff}},
	}
	for _, m := range sent {
		if err := a.Conn.WriteMessage(m.Type, m.Data); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	for _, want := range sent {
		got := b.Next(t)
		if got.Err != nil || got.Type != want.Type || string(got.Data) != string(want.Data) {
			t.Errorf("got type=%d data=%v err=%v, want type=%d data=%v", got.Type, got.Data, got.Err, want.Type, want.Data)
		}
	}
	a.ExpectNone(t, 100*time.Millisecond)
}

func TestServeConn_OversizedMessageClosesSender(t *testing.T) {
	h := startHub(t)
	url := serveHub(t, h)
	a, b := wstest.Dial(t, url, nil), wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, a, b)

	if err := a.Conn.WriteMessage(websocket.TextMessage, make([]byte, maxMessageSize+1)); err != nil {
		t.Fatalf("write: %v", err)
	}

	if msg := a.Next(t); !websocket.IsCloseError(msg.Err, websocket.CloseMessageTooBig) {
		t.Fatalf("got err %v, want close 1009", msg.Err)
	}
	b.ExpectNone(t, 100*time.Millisecond)
}

func TestServeConn_UnexpectedCloseUnregisters(t *testing.T) {
	h := startHub(t)
	url := serveHub(t, h)
	a, b := wstest.Dial(t, url, nil), wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, a, b)

	msg := websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "boom")
	if err := a.Conn.WriteMessage(websocket.CloseMessage, msg); err != nil {
		t.Fatalf("write close: %v", err)
	}

	// The remaining clients keep relaying after a leaves.
	c := wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, b, c)
}

func TestStop_SendsCloseFrame(t *testing.T) {
	h := New(zap.NewNop())
	go h.Run()
	url := serveHub(t, h)
	a, b := wstest.Dial(t, url, nil), wstest.Dial(t, url, nil)
	wstest.WaitConnected(t, a, b)

	h.Stop()

	for _, c := range []*wstest.Client{a, b} {
		if msg := c.Next(t); !websocket.IsCloseError(msg.Err, websocket.CloseGoingAway) {
			t.Errorf("got err %v, want close 1001", msg.Err)
		}
	}
}
