package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"

	"github.com/gorilla/websocket"
	"github.com/grandcat/zeroconf"
)

const (
	zeroconfType   = "_ship._tcp"
	zeroconfDomain = "local."
)

func discoverDNS(results <-chan *zeroconf.ServiceEntry) {
	for entry := range results {
		fmt.Printf("%+v\n", entry)
	}
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func createCertificate(isCA bool, hosts ...string) tls.Certificate {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	// fmt.Println(out.String())

	out.Reset()
	pem.Encode(out, pemBlockForKey(priv))
	// fmt.Println(out.String())

	// tls.LoadX509KeyPair(certFile, keyFile)
	tlsCert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	return tlsCert
}

func SelfSigned(uri string) (*websocket.Conn, error) {
	tlsClientCert := createCertificate(false)
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			// RootCAs:      caCertPool,
			Certificates: []tls.Certificate{tlsClientCert},
		},
	}

	conn, resp, err := dialer.Dial(uri, http.Header{})
	fmt.Println(resp)

	return conn, err
}

func server(host, port string) {
	tlsServerCert := createCertificate(false, host)
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{tlsServerCert},
	}

	addr := host + ":" + port
	srv := http.Server{
		Addr:      addr,
		TLSConfig: &tlsConfig,
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatal(err)
	}

	defer ln.Close()

	tlsListener := tls.NewListener(ln, &tlsConfig)
	srv.Serve(tlsListener)
}

func client(uri string) {
	tlsClientCert := createCertificate(false)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsClientCert},
		InsecureSkipVerify: true,
		// RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	resp, err := client.Get(uri)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(resp.Status)
}

func connect(uri string) {
	tlsClientCert := createCertificate(false, "")
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsClientCert},
		InsecureSkipVerify: insecure,
		ServerName:         srvName,
	}
	tlsConfig.BuildNameToCertificate()

	conn, err := tls.Dial("tcp", uri, tlsConfig)
	if err != nil {
		panic("failed to connect: " + err.Error())
	}
	println("done")

	conn.Close()
}

var (
	host     string
	port     string
	srvName  string
	insecure bool
)

func main() {
	// host := "localhost"
	// port := "8443"
	// go server(host, port)

	// time.Sleep(time.Second)

	// uri := "https://" + host + ":" + port
	// client(uri)

	flag.StringVar(&srvName, "server", "", "server")
	flag.StringVar(&host, "host", "", "host")
	flag.StringVar(&port, "port", "4711", "port")
	flag.BoolVar(&insecure, "insecure", false, "skip certificate verification")
	flag.Parse()

	if host != "" {
		uri := host + ":" + port
		connect(uri)
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go discoverDNS(entries)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	if err := resolver.Browse(ctx, zeroconfType, zeroconfDomain, entries); err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
}
