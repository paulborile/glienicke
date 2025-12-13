package integration

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/relay"
)

func TestWSS_StartTLSMethodExists(t *testing.T) {
	r := relay.New(nil)
	defer func() {
		if r := recover(); r != nil {
			t.Log("StartTLS method not implemented")
			t.FailNow()
		}
	}()
	_ = r.StartTLS(":8443", "test-cert.pem", "test-key.pem")
	t.Log("StartTLS method exists")
}

func TestWSS_EndToEndConnection(t *testing.T) {
	// Generate test certificate and key
	certFile, keyFile, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Create in-memory store
	store := memory.New()
	defer store.Close()

	// Create relay
	r := relay.New(store)
	defer r.Close()

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start TLS server in goroutine
	go func() {
		if err := r.StartTLS(addr, certFile, keyFile); err != nil {
			t.Logf("TLS server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create TLS dialer for client
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // Accept self-signed cert for test
	}

	// Connect client using WSS
	wssURL := "wss://" + addr + "/"
	client, err := testutil.NewWSClientWithDialer(wssURL, dialer)
	if err != nil {
		t.Fatalf("Failed to connect to WSS server: %v", err)
	}
	defer client.Close()

	// Test basic message exchange - create a valid signed test event
	testEvent, _ := testutil.MustNewTestEvent(1, "Hello WSS!", nil)

	// Send event
	if err := client.SendEvent(testEvent); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Wait for OK response
	accepted, message, err := client.ExpectOK(testEvent.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive OK response: %v", err)
	}

	if !accepted {
		t.Errorf("Event was rejected: %s", message)
	}

	t.Log("WSS end-to-end test passed")
}

func generateTestCertificate() (certFile, keyFile string, err error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	// Write certificate to file
	certFile = "test-cert.pem"
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", err
	}

	// Write private key to file
	keyFile = "test-key.pem"
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", err
	}

	return certFile, keyFile, nil
}
