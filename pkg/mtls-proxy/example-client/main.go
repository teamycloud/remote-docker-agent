package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	// Command-line flags
	var (
		proxyAddr  = flag.String("proxy", "localhost:8443", "Proxy address")
		clientCert = flag.String("cert", "", "Client certificate path")
		clientKey  = flag.String("key", "", "Client private key path")
		caCert     = flag.String("ca", "", "CA certificate path")
		connectID  = flag.String("connect-id", "", "Connect ID for routing")
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
	if *connectID == "" {
		log.Fatal("--connect-id is required")
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
	}

	// Connect to proxy
	log.Printf("Connecting to proxy at %s...", *proxyAddr)
	conn, err := tls.Dial("tcp", *proxyAddr, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected. Sending connect_id: %s", *connectID)

	// Send connect_id
	if _, err := fmt.Fprintf(conn, "%s\n", *connectID); err != nil {
		log.Fatalf("Failed to send connect_id: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	response := strings.TrimSpace(string(buf[:n]))
	log.Printf("Proxy response: %s", response)

	if !strings.HasPrefix(response, "OK") {
		log.Fatalf("Proxy returned error: %s", response)
	}

	log.Println("Connection established successfully!")
	log.Println("You can now communicate with the backend server")
	log.Println("Type messages to send to backend (Ctrl+C to exit)")

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
