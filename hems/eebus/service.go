package eebus

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andig/evcc/hems/eebus/ship"
	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
	"github.com/mitchellh/mapstructure"
)

const shipScheme = "wss://"

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
	Conn *ship.Connection
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

// Connector is the connector used for establishing new websocket connections
var Connector = defaultWebsocketConnector

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

// Connect connects to the service endpoint and performs handshake
func (ss *Service) Connect() error {
	conn, err := Connector(ss.URI)
	if err != nil {
		return err
	}

	ss.Conn = ship.New(conn)

	return ss.Conn.Connect()
}

// Close closes the service connection
func (ss *Service) Close() error {
	return ss.Conn.Close()
}
