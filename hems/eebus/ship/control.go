package ship

import (
	"errors"
)

const (
	CmiTypeControl = 1
)

const (
	ProtocolHandshakeFormatJSON = "JSON-UTF8"

	ProtocolHandshakeTypeAnnounceMax = "announceMax"
	ProtocolHandshakeTypeSelect      = "select"

	SubProtocol = "ship"
)

type ProtocolHandshake struct {
	HandshakeType string   `json:"handshakeType"`
	Version       Version  `json:"version"`
	Formats       []string `json:"formats"`
}

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

const (
	CmiProtocolHandshakeErrorUnexpectedMessage = 2
)

type CmiProtocolHandshakeError struct {
	Error int `json:"error"`
}

func (c *Connection) protocolHandshake() error {
	req := CmiHandshakeMsg{
		ProtocolHandshake: ProtocolHandshake{
			HandshakeType: ProtocolHandshakeTypeAnnounceMax,
			Version:       Version{Major: 1, Minor: 0},
			Formats:       []string{ProtocolHandshakeFormatJSON},
		},
	}

	err := c.writeJSON(req)

	var resp CmiHandshakeMsg
	if err == nil {
		err = c.readJSON(&resp)
	}

	if err == nil {
		if resp.ProtocolHandshake.HandshakeType != ProtocolHandshakeTypeSelect ||
			len(resp.ProtocolHandshake.Formats) != 1 ||
			resp.ProtocolHandshake.Formats[0] != ProtocolHandshakeFormatJSON {
			msg := CmiProtocolHandshakeError{
				Error: CmiProtocolHandshakeErrorUnexpectedMessage,
			}
			_ = c.writeJSON(msg)
			err = errors.New("invalid protocol handshake response")
		} else {
			// send selection back to server
			err = c.writeJSON(resp)
		}
	}

	return err
}
