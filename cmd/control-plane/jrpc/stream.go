package jrpc

import "github.com/gofiber/websocket/v2"

// Wrapper around web socket for JSON RPC used for communicating with rVPN clients connecting to rVPN servers

// A ObjectStream is a jsonrpc2.ObjectStream that uses a WebSocket to
// send and receive JSON-RPC 2.0 objects
type ObjectStream struct {
	conn *websocket.Conn
}

// NewObjectStream creates a new jsonrpc2.ObjectStream for sending and
// receiving JSON-RPC 2.0 objects over a WebSocket
func NewObjectStream(conn *websocket.Conn) ObjectStream {
	return ObjectStream{conn: conn}
}

// WriteObject implements jsonrpc2.ObjectStream
func (t ObjectStream) WriteObject(obj interface{}) error {
	return t.conn.WriteJSON(obj)
}

// ReadObject implements jsonrpc2.ObjectStream
func (t ObjectStream) ReadObject(v interface{}) error {
	err := t.conn.ReadJSON(v)
	// TODO: unwrap error if it is connection closed to be less verbose
	// see https://github.com/sourcegraph/jsonrpc2/blob/master/websocket/stream.go
	return err
}

// Close implements jsonrpc2.ObjectStream
func (t ObjectStream) Close() error {
	return t.conn.Close()
}
