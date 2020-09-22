package ship

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
	"github.com/mitchellh/mapstructure"
)

const (
	shipScheme          = "wss://"
	cmiReadWriteTimeout = 10 * time.Second
)

// ServiceDescription contains the ship service parameters
type ServiceDescription struct {
	Model, Brand string
	SKI          string
	Register     bool
	Path         string
	ID           string
}

// Service is the ship service
type Service struct {
	ServiceDescription
	URI  string
	conn *websocket.Conn
}

// NewFromDNSEntry creates ship service from its DNS definition
func NewFromDNSEntry(zc *zeroconf.ServiceEntry) (*Service, error) {
	ss := Service{}

	txtM := make(map[string]interface{})
	for _, txtE := range zc.Text {
		split := strings.SplitN(txtE, "=", 2)
		if len(split) == 2 {
			txtM[split[0]] = split[1]
		}
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Result:           &ss.ServiceDescription,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err == nil {
		err = decoder.Decode(txtM)
	}

	ss.URI = baseURIFromDNS(zc) + ss.ServiceDescription.Path

	return &ss, err
}

// baseURIFromDNS returns the service URI
func baseURIFromDNS(zc *zeroconf.ServiceEntry) string {
	uri := shipScheme + zc.HostName
	if zc.Port != 443 {
		uri += fmt.Sprintf(":%d", zc.Port)
	}
	fmt.Println("uri: " + uri)

	return uri
}

func (ss *Service) writeBinary(msg []byte) error {
	err := ss.conn.SetWriteDeadline(time.Now().Add(cmiReadWriteTimeout))
	if err == nil {
		ss.conn.WriteMessage(websocket.BinaryMessage, msg)
	}
	return err
}

func (ss *Service) writeJSON(jsonMsg interface{}) error {
	msg, err := json.Marshal(jsonMsg)
	if err != nil {
		return err
	}

	return ss.writeBinary(msg)
}

func (ss *Service) readBinary() ([]byte, error) {
	err := ss.conn.SetReadDeadline(time.Now().Add(cmiReadWriteTimeout))
	if err != nil {
		return nil, err
	}

	typ, msg, err := ss.conn.ReadMessage()

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

// DefaultConnector is the connector used for establishing new websocket connections
var DefaultConnector = defaultWebsocketConnector

func defaultWebsocketConnector(uri string) (*websocket.Conn, error) {
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
		// TLSClientConfig:  &tls.Config{
		// 	RootCAs:      caCertPool,
		// 	Certificates: []tls.Certificate{tlsClientCert},
		// }
	}

	conn, resp, err := dialer.Dial(uri, http.Header{})
	fmt.Println(resp)

	return conn, err
}

// Connect connects to the service endpoint and performs protocol handshake
func (ss *Service) Connect() error {
	conn, err := DefaultConnector(ss.URI)
	if err != nil {
		return err
	}
	ss.conn = conn

	// handshake
	err = ss.handshake()
	if err == nil {
		err = ss.hello()
	}
	if err == nil {
		err = ss.protocolHandshake()
	}

	// close connection if handshake or hello fails
	if err != nil {
		_ = ss.conn.Close()
	}

	return err
}

// Close closes the service connection
func (ss *Service) Close() error {
	err := ss.close()
	_ = ss.conn.Close()
	return err
}
