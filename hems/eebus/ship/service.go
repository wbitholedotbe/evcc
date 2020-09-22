package ship

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
	"github.com/mitchellh/mapstructure"
)

const (
	shipScheme = "wss://"

	CmiReadWriteTimeout         = 10 * time.Second
	CmiHelloInitTimeout         = 60 * time.Second
	CmiHelloProlongationTimeout = 30 * time.Second

	CmiTypeInit    = 0
	CmiTypeControl = 1
	CmiTypeData    = 2
	CmiTypeEnd     = 3

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

const (
	ProtocolHandshakeFormatJSON = "JSON-UTF8"

	ProtocolHandshakeTypeAnnounceMax = "announceMax"
	ProtocolHandshakeTypeSelect      = "select"
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

type Service struct {
	*zeroconf.ServiceEntry
	Model, Brand string
	SKI          string
	Register     bool
	Path         string
	ID           string
	*websocket.Conn
}

func NewFromDNSEntry(zc *zeroconf.ServiceEntry) (*Service, error) {
	ss := Service{
		ServiceEntry: zc,
	}

	txtM := make(map[string]interface{})
	for _, txtE := range zc.Text {
		split := strings.SplitN(txtE, "=", 2)
		if len(split) == 2 {
			txtM[split[0]] = split[1]
		}
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Result:           &ss,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err == nil {
		err = decoder.Decode(txtM)
	}

	return &ss, err
}

func (ss *Service) writeBinary(msg []byte) error {
	ss.SetWriteDeadline(time.Now().Add(CmiReadWriteTimeout))
	return ss.WriteMessage(websocket.BinaryMessage, msg)
}

func (ss *Service) writeJSON(jsonMsg interface{}) error {
	msg, err := json.Marshal(jsonMsg)
	if err != nil {
		return err
	}

	return ss.writeBinary(msg)
}

func (ss *Service) readBinary() ([]byte, error) {
	ss.SetReadDeadline(time.Now().Add(CmiReadWriteTimeout))
	typ, msg, err := ss.ReadMessage()

	if err == nil && typ != websocket.BinaryMessage {
		err = fmt.Errorf("invalid message type: %d", typ)
	}

	return msg, err
}

func (ss *Service) readJSON(jsonMsg interface{}) error {
	msg, err := ss.readBinary()
	if err == nil {
		err = json.Unmarshal(msg, &jsonMsg)
	}

	return err
}

func (ss *Service) handshake() error {
	init := []byte{CmiTypeInit, CmiTypeInit}

	// CMI_STATE_CLIENT_SEND
	if err := ss.writeBinary(init); err != nil {
		return err
	}

	// CMI_STATE_CLIENT_EVALUATE
	msg, err := ss.readBinary()
	if err != nil {
		return err
	}
	log.Printf("recv: %0 x", msg)

	if bytes.Compare(init, msg) != 0 {
		return fmt.Errorf("invalid init response: %0 x", msg)
	}

	return nil
}

func (ss *Service) hello() error {
	errC := make(chan error)

	// always send READY
	go func(errC chan<- error) {
		msg := CmiHelloMsg{
			ConnectionHello{Phase: CmiHelloPhaseReady},
		}

		if err := ss.writeJSON(msg); err != nil {
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
				err := ss.readJSON(&msg)
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

func (ss *Service) protocolHandshake() error {
	req := CmiHandshakeMsg{
		ProtocolHandshake: ProtocolHandshake{
			HandshakeType: ProtocolHandshakeTypeAnnounceMax,
			Version:       Version{Major: 1, Minor: 0},
			Formats:       []string{ProtocolHandshakeFormatJSON},
		},
	}

	var resp CmiHandshakeMsg
	err := ss.writeJSON(req)
	if err == nil {
		err = ss.readJSON(&resp)
	}

	if err == nil {
		if resp.ProtocolHandshake.HandshakeType != ProtocolHandshakeTypeSelect ||
			len(resp.ProtocolHandshake.Formats) != 1 ||
			resp.ProtocolHandshake.Formats[0] != ProtocolHandshakeFormatJSON {
			msg := CmiProtocolHandshakeError{
				Error: CmiProtocolHandshakeErrorUnexpectedMessage,
			}
			_ = ss.writeJSON(msg)
			err = errors.New("invalid protocol handshake response")
		} else {
			// send selection back to server
			err = ss.writeJSON(resp)
		}
	}

	return err
}

func (ss *Service) Connect() error {
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
		// TLSClientConfig:  &tls.Config{
		// 	RootCAs:      caCertPool,
		// 	Certificates: []tls.Certificate{tlsClientCert},
		// }
	}

	uri := shipScheme + ss.HostName
	if ss.Port != 443 {
		uri += fmt.Sprintf(":%d", ss.Port)
	}
	uri += ss.Path
	fmt.Println("uri: " + uri)

	conn, resp, err := dialer.Dial(uri, http.Header{})
	if err != nil {
		return err
	}

	ss.Conn = conn
	fmt.Println(resp)

	err = ss.handshake()
	if err == nil {
		if err = ss.hello(); err != nil {
			// send ABORT if hello fails
			_ = ss.writeJSON(CmiHelloMsg{
				ConnectionHello{Phase: CmiHelloPhaseAborted},
			})
		}
	}

	if err == nil {
		err = ss.protocolHandshake()
	}

	// close connection if handshake or hello fails
	if err != nil {
		_ = ss.Close()
	}

	return err
}
