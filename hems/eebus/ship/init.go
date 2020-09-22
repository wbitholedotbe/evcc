package ship

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	CmiTypeInit = 0

	CmiHelloInitTimeout         = 60 * time.Second
	CmiHelloProlongationTimeout = 30 * time.Second

	CmiHelloPhasePending = "pending"
	CmiHelloPhaseReady   = "ready"
	CmiHelloPhaseAborted = "aborted"
)

type CmiHelloMsg struct {
	ConnectionHello `json:"connectionHello"`
}

type ConnectionHello struct {
	Phase               string `json:"phase"`
	Waiting             int    `json:"waiting,omitempty"`
	ProlongationRequest bool   `json:"prolongationRequest,omitempty"`
}

type CmiHandshakeMsg struct {
	ProtocolHandshake `json:"messageProtocolHandshake"`
}

func (c *Connection) handshake() error {
	init := []byte{CmiTypeInit, CmiTypeInit}

	// CMI_STATE_CLIENT_SEND
	if err := c.writeBinary(init); err != nil {
		return err
	}

	// CMI_STATE_CLIENT_EVALUATE
	msg, err := c.readBinary()
	if err != nil {
		return err
	}
	log.Printf("recv: %0 x", msg)

	if bytes.Compare(init, msg) != 0 {
		return fmt.Errorf("invalid init response: %0 x", msg)
	}

	return nil
}

func (c *Connection) hello() (err error) {
	// send ABORT if hello fails
	defer func() {
		if err != nil {
			_ = c.writeJSON(CmiHelloMsg{
				ConnectionHello{Phase: CmiHelloPhaseAborted},
			})
		}
	}()

	// always send READY
	errC := make(chan error)
	go func(errC chan<- error) {
		msg := CmiHelloMsg{
			ConnectionHello{Phase: CmiHelloPhaseReady},
		}

		if err := c.writeJSON(msg); err != nil {
			errC <- fmt.Errorf("hello send failed: %w", err)
		}
	}(errC)

	readC := make(chan CmiHelloMsg, 1)
	closeC := make(chan struct{})
	defer close(closeC)

	// read loop
	go func(readC chan<- CmiHelloMsg, closeC chan struct{}, errC chan error) {
		var msg CmiHelloMsg
		for {
			select {
			case <-closeC:
				return
			default:
				err := c.readJSON(&msg)
				if err == nil {
					readC <- msg
				} else {
					errC <- fmt.Errorf("hello read failed: %w", err)
				}
			}
		}
	}(readC, closeC, errC)

	timer := time.NewTimer(CmiHelloInitTimeout)
	for {
		select {
		case msg := <-readC:
			log.Printf("hello recv: %+v", msg)

			switch msg.ConnectionHello.Phase {
			case "":
				return errors.New("invalid hello response")
			case CmiHelloPhaseAborted:
				return errors.New("hello aborted by peer")
			case CmiHelloPhaseReady:
				return nil
			case CmiHelloPhasePending:
				if msg.ConnectionHello.ProlongationRequest {
					timer = time.NewTimer(CmiHelloProlongationTimeout)
				}
			}
		case err := <-errC:
			return err
		case <-timer.C:
			return errors.New("hello timeout")
		}
	}
}
