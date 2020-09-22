package ship

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

const cmiReadWriteTimeout = 10 * time.Second

type Connection struct {
	conn *websocket.Conn
}

func New(conn *websocket.Conn) (c *Connection) {
	return &Connection{
		conn: conn,
	}
}

// Connect performs the connection handshake
func (c *Connection) Connect() error {
	err := c.handshake()
	if err == nil {
		err = c.hello()
	}
	if err == nil {
		err = c.protocolHandshake()
	}

	// close connection if handshake or hello fails
	if err != nil {
		_ = c.conn.Close()
	}

	return err
}

func (c *Connection) writeBinary(msg []byte) error {
	err := c.conn.SetWriteDeadline(time.Now().Add(cmiReadWriteTimeout))
	if err == nil {
		c.conn.WriteMessage(websocket.BinaryMessage, msg)
	}
	return err
}

func (c *Connection) writeJSON(jsonMsg interface{}) error {
	msg, err := json.Marshal(jsonMsg)
	if err != nil {
		return err
	}

	return c.writeBinary(msg)
}

func (c *Connection) readBinary() ([]byte, error) {
	err := c.conn.SetReadDeadline(time.Now().Add(cmiReadWriteTimeout))
	if err != nil {
		return nil, err
	}

	typ, msg, err := c.conn.ReadMessage()

	if err == nil && typ != websocket.BinaryMessage {
		err = fmt.Errorf("invalid message type: %d", typ)
	}

	return msg, err
}

func (c *Connection) readJSON(jsonMsg interface{}) error {
	msg, err := c.readBinary()
	if err == nil {
		err = json.Unmarshal(msg, &jsonMsg)
	}

	return err
}

// Close closes the service connection
func (c *Connection) Close() error {
	err := c.close()
	_ = c.conn.Close()
	return err
}
