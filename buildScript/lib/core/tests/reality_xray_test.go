//go:build with_utls

package tls

import (
	"bufio"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
)

// Set XRAY_TEST_BINARY to an official Xray binary. Everything else, including
// the REALITY TLS camouflage destination and HTTP destination, stays on loopback.
func TestRealityXrayEndToEnd(t *testing.T) {
	binary := os.Getenv("XRAY_TEST_BINARY")
	if binary == "" {
		t.Skip("XRAY_TEST_BINARY is required for the external-core integration test")
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &certKey.PublicKey, certKey)
	if err != nil {
		t.Fatal(err)
	}
	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	target.TLS = &stdtls.Config{
		MinVersion:   stdtls.VersionTLS13,
		Certificates: []stdtls.Certificate{{Certificate: [][]byte{der}, PrivateKey: certKey}},
	}
	target.StartTLS()
	defer target.Close()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "REALITY end-to-end verified")
	}))
	defer destination.Close()
	serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var uuid [16]byte
	if _, err = rand.Read(uuid[:]); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := map[string]any{
		"log": map[string]any{"loglevel": "debug"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": "vless",
			"settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])}}},
			"streamSettings": map[string]any{
				"network": "tcp", "security": "reality",
				"realitySettings": map[string]any{
					"target": target.Listener.Addr().String(), "serverNames": []string{"localhost"},
					"privateKey": base64.RawURLEncoding.EncodeToString(serverKey.Bytes()), "shortIds": []string{"1234"},
				},
			},
		}},
		"outbounds": []any{map[string]any{
			"protocol": "freedom",
			"settings": map[string]any{"finalRules": []any{map[string]any{
				"action": "allow", "network": "tcp", "ip": []string{"127.0.0.1/32"},
				"port": fmt.Sprint(destination.Listener.Addr().(*net.TCPAddr).Port),
			}}},
		}},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "server.json")
	if err = os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.Create(filepath.Join(dir, "xray.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "run", "-c", configPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
		if t.Failed() {
			log, _ := os.ReadFile(logFile.Name())
			t.Logf("Xray logs:\n%s", log)
		}
	}()
	var conn net.Conn
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		conn, err = net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	client, err := NewRealityClient(context.Background(), "localhost", option.OutboundTLSOptions{
		ServerName: "localhost",
		UTLS:       &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
		Reality:    &option.OutboundRealityOptions{Enabled: true, ShortID: "1234", PublicKey: base64.RawURLEncoding.EncodeToString(serverKey.PublicKey().Bytes())},
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnel, err := client.ClientHandshake(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	destPort := destination.Listener.Addr().(*net.TCPAddr).Port
	request := append([]byte{0}, uuid[:]...)
	request = append(request, 0, 1, byte(destPort>>8), byte(destPort), 1, 127, 0, 0, 1)
	request = append(request, []byte("GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")...)
	if _, err = tunnel.Write(request); err != nil {
		t.Fatal(err)
	}
	var responseHeader [2]byte
	if _, err = io.ReadFull(tunnel, responseHeader[:]); err != nil {
		t.Fatal(err)
	}
	if responseHeader != [2]byte{} {
		t.Fatalf("unexpected VLESS response: %x", responseHeader)
	}
	response, err := http.ReadResponse(bufio.NewReader(tunnel), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || string(body) != "REALITY end-to-end verified" {
		t.Fatalf("HTTP tunnel failed: status=%d body=%q error=%v", response.StatusCode, body, err)
	}
}
