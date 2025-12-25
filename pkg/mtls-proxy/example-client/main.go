package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io"
	"log"
	"os"
)

func main() {
	// Command-line flags
	var (
		proxyAddr  = flag.String("proxy", "localhost:8443", "Proxy address")
		clientCert = flag.String("cert", "", "Client certificate path")
		clientKey  = flag.String("key", "", "Client private key path")
		caCert     = flag.String("ca", "", "CA certificate path")
		sni        = flag.String("sni", "", "SNI hostname with connectID (e.g., connect-id-123.connect.tinyscale.com)")
	)

	flag.Parse()

	// Validate required flags
	if *clientCert == "" {
		log.Fatal("--cert is required")
	}
	if *clientKey == "" {
		log.Fatal("--key is required")
	}
	if *caCert == "" {
		log.Fatal("--ca is required")
	}
	if *sni == "" {
		log.Fatal("--sni is required (e.g., connect-id-123.connect.tinyscale.com)")
	}

	// Load client certificate
	cert, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
	if err != nil {
		log.Fatalf("Failed to load client certificate: %v", err)
	}

	// Load CA certificate
	caCertBytes, err := os.ReadFile(*caCert)
	if err != nil {
		log.Fatalf("Failed to read CA certificate: %v", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertBytes) {
		log.Fatal("Failed to parse CA certificate")
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		ServerName:   *sni, // Set SNI with connectID
	}

	// Connect to proxy
	log.Printf("Connecting to proxy at %s with SNI: %s", *proxyAddr, *sni)
	conn, err := tls.Dial("tcp", *proxyAddr, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	log.Println("Connected successfully!")

	// Start bidirectional communication
	go func() {
		// Copy from stdin to connection
		if _, err := io.Copy(conn, os.Stdin); err != nil {
			log.Printf("Error copying from stdin: %v", err)
		}
	}()

	// Copy from connection to stdout
	if _, err := io.Copy(os.Stdout, conn); err != nil {
		log.Printf("Error copying to stdout: %v", err)
	}

	log.Println("Connection closed")
}
