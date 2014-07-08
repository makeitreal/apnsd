package apns

import (
	"bytes"
	"io"
	"net"
)

const (
	Port              = "2195"
	ProductionGateway = "gateway.push.apple.com"
	SandboxGateway    = "gateway.sandbox.push.apple.com"
)

type Connection struct {
	net.Conn
}

func NewConnection(conn net.Conn) *Connection {
	c := &Connection{
		conn,
	}
	return c
}

func (c *Connection) WriteMsg(msg *Msg) error {
	var b bytes.Buffer
	if err := msg.write(&b); err != nil {
		return err
	}
	if _, err := c.Write(b.Bytes()); err != nil {
		return err
	}
	return nil
}

func (c *Connection) ReadErrorMsg() (*ErrorMsg, error) {
	b := make([]byte, 6)
	_, err := c.Read(b)
	if err != nil && err != io.EOF {
		return nil, err
	}

	e := &ErrorMsg{}
	e.Read(bytes.NewReader(b))

	return e, nil
}
