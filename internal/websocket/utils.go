package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

// WriteJSON sends a standardized JSON message over the WebSocket.
// It wraps the data into a standard ResponsePayload struct if not already formatted,
// or sends custom payload if data implements custom structure?
// Actually, to enforce schema, this helper should wrap data.
func WriteJSON(conn *websocket.Conn, event Event, data interface{}) error {
	payload := ResponsePayload{
		Event: event,
		Data:  data,
	}

	// Set write deadline for robustness.
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(payload)
}

// WriteError sends a standardized error message over the WebSocket.
func WriteError(conn *websocket.Conn, err string) error {
	payload := ResponsePayload{
		Event: EventError,
		Error: err,
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(payload)
}

// ReadJSON reads and decodes a message into the provided structure.
// It sets a read deadline.
func ReadJSON(conn *websocket.Conn, v interface{}) error {
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute)) // Reset keep-alive deadline
	return conn.ReadJSON(v)
}

// WriteRawJSON sends a pre-formatted payload (e.g. for existing handlers needing backward compat).
func WriteRawJSON(conn *websocket.Conn, v interface{}) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}
