// Package wstest provides WebSocket client helpers for tests.
package wstest

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	Timeout      = 2 * time.Second
	syncMessage  = "wstest-sync"
	syncInterval = 20 * time.Millisecond
)

// Message is one read result. Err is set on the last Message before the
// channel closes.
type Message struct {
	Type int
	Data []byte
	Err  error
}

// Client is a dialed connection with a single background reader, since a
// timed-out read breaks a gorilla connection.
type Client struct {
	Conn     *websocket.Conn
	Received <-chan Message
}

// URL converts an httptest server URL to its ws:// equivalent.
func URL(serverURL, path string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + path
}

// Dial connects to url and starts the background reader. The connection is
// closed when the test ends.
func Dial(t *testing.T, url string, header http.Header) *Client {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	received := make(chan Message, 64)
	go func() {
		defer close(received)
		for {
			messageType, data, err := conn.ReadMessage()
			received <- Message{Type: messageType, Data: data, Err: err}
			if err != nil {
				return
			}
		}
	}()

	return &Client{Conn: conn, Received: received}
}

// WaitConnected sends sync messages from sender until one reaches receiver,
// which proves both are registered with the hub.
func WaitConnected(t *testing.T, sender, receiver *Client) {
	t.Helper()

	deadline := time.After(Timeout)
	for {
		if err := sender.Conn.WriteMessage(websocket.TextMessage, []byte(syncMessage)); err != nil {
			t.Fatalf("write sync: %v", err)
		}
		select {
		case msg := <-receiver.Received:
			if msg.Err != nil {
				t.Fatalf("receiver closed while syncing: %v", msg.Err)
			}
			return
		case <-time.After(syncInterval):
		case <-deadline:
			t.Fatal("clients never connected")
		}
	}
}

// Next returns the next message that is not a sync message, failing the test
// if none arrives in time.
func (c *Client) Next(t *testing.T) Message {
	t.Helper()

	deadline := time.After(Timeout)
	for {
		select {
		case msg, ok := <-c.Received:
			if !ok {
				t.Fatal("connection reader stopped")
			}
			if msg.Err == nil && string(msg.Data) == syncMessage {
				continue
			}
			return msg
		case <-deadline:
			t.Fatal("timed out waiting for message")
		}
	}
}

// ExpectNone fails the test if a non-sync message arrives within wait.
func (c *Client) ExpectNone(t *testing.T, wait time.Duration) {
	t.Helper()

	deadline := time.After(wait)
	for {
		select {
		case msg, ok := <-c.Received:
			if !ok {
				return
			}
			if msg.Err == nil && string(msg.Data) == syncMessage {
				continue
			}
			t.Fatalf("unexpected message: type=%d data=%q err=%v", msg.Type, msg.Data, msg.Err)
		case <-deadline:
			return
		}
	}
}
