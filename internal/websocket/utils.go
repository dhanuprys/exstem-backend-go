package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

// WriteTyped sends a strongly-typed response payload over the WebSocket.
func WriteTyped(conn *websocket.Conn, v interface{}) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

// WriteError sends a typed ErrorResponse over the WebSocket.
func WriteError(conn *websocket.Conn, errMsg string) error {
	return WriteTyped(conn, ErrorResponse{
		Event: EventError,
		Error: errMsg,
	})
}

// ReadJSON reads and decodes a message into the provided structure.
// It sets a read deadline.
func ReadJSON(conn *websocket.Conn, v interface{}) error {
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	return conn.ReadJSON(v)
}
