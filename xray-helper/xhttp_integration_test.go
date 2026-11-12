package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/core"
	"golang.org/x/net/proxy"
)

func reservePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func certificate(t *testing.T) (tls.Certificate, string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
}

// A real TCP forwarding hop stands in for the existing sing-box protected
// mapping. No remote VPS, real credentials, or external HTTP service is used.
func forwardTCP(t *testing.T, address string) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			client, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()
				remote, err := net.DialTimeout("tcp", address, 3*time.Second)
				if err != nil {
					return
				}
				defer remote.Close()
				go func() { io.Copy(remote, client); remote.Close() }()
				io.Copy(client, remote)
			}()
		}
	}()
	return l
}

func TestXhttpThroughProtectedMapping(t *testing.T) {
	binary := os.Getenv("XHTTP_TEST_BINARY")
	templateDir := os.Getenv("XHTTP_TEST_CONFIG_DIR")
	if binary == "" || templateDir == "" {
		t.Skip("set XHTTP_TEST_BINARY and XHTTP_TEST_CONFIG_DIR after the Android JVM tests")
	}
	for _, tc := range []struct {
		security, mode string
		obfuscate      bool
	}{
		{"none", "packet-up", false},
		{"tls", "auto", false},
		{"tls", "stream-up", false},
		{"tls", "stream-one", false},
		{"reality", "auto", false},
		{"reality", "auto", true},
		{"reality", "auto-legacy-padding", true},
	} {
		t.Run(fmt.Sprintf("%s/%s/obfs=%t", tc.security, tc.mode, tc.obfuscate), func(t *testing.T) {
			cert, certPEM, keyPEM := certificate(t)
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
			target.EnableHTTP2 = true
			target.Config.ErrorLog = log.New(io.Discard, "", 0)
			target.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}
			target.StartTLS()
			defer target.Close()
			payload := bytes.Repeat([]byte("XHTTP verified\n"), 4096)
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
			defer destination.Close()
			data, err := os.ReadFile(filepath.Join(templateDir, "generated-tls.json"))
			if err != nil {
				t.Fatal(err)
			}
			var clientConfig map[string]any
			if err = json.Unmarshal(data, &clientConfig); err != nil {
				t.Fatal(err)
			}
			out := clientConfig["outbounds"].([]any)[0].(map[string]any)
			endpoint := out["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)
			userID := endpoint["users"].([]any)[0].(map[string]any)["id"]
			serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			extra := map[string]any{"noSSEHeader": true}
			if tc.obfuscate {
				extra["xPaddingObfsMode"] = true
				extra["xPaddingMethod"] = "tokenish"
			}
			xhttp := map[string]any{"host": "cdn.example", "path": "/transport/", "mode": "auto", "extra": extra}
			serverStream := map[string]any{"network": "xhttp", "security": tc.security, "xhttpSettings": xhttp}
			if tc.security == "reality" {
				serverStream["realitySettings"] = map[string]any{
					"target": target.Listener.Addr().String(), "serverNames": []string{"localhost"},
					"privateKey": base64.RawURLEncoding.EncodeToString(serverKey.Bytes()), "shortIds": []string{"1234"},
				}
			} else if tc.security == "tls" {
				serverStream["tlsSettings"] = map[string]any{"alpn": []string{"h2", "http/1.1"}, "certificates": []any{map[string]any{
					"certificate": strings.Split(certPEM, "\n"), "key": strings.Split(keyPEM, "\n"),
				}}}
			}
			serverPort := reservePort(t)
			destPort := destination.Listener.Addr().(*net.TCPAddr).Port
			serverConfig := map[string]any{
				"log": map[string]any{"loglevel": "none"},
				"inbounds": []any{map[string]any{
					"listen": "127.0.0.1", "port": serverPort, "protocol": "vless", "streamSettings": serverStream,
					"settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": userID}}},
				}},
				"outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{
					"finalRules": []any{map[string]any{"action": "allow", "network": "tcp", "ip": []string{"127.0.0.1/32"}, "port": fmt.Sprint(destPort)}},
				}}},
			}
			serverJSON, _ := json.Marshal(serverConfig)
			if serverBinary := os.Getenv("XHTTP_TEST_SERVER_BINARY"); serverBinary != "" {
				serverPath := filepath.Join(t.TempDir(), "server.json")
				if err = os.WriteFile(serverPath, serverJSON, 0600); err != nil {
					t.Fatal(err)
				}
				serverCommand := exec.Command(serverBinary, "run", "-c", serverPath)
				if err = serverCommand.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() { serverCommand.Process.Kill(); serverCommand.Wait() }()
				for deadline := time.Now().Add(5 * time.Second); ; {
					probe, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", serverPort), 100*time.Millisecond)
					if dialErr == nil {
						probe.Close()
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("external Xray listener did not start")
					}
					time.Sleep(20 * time.Millisecond)
				}
			} else {
				server, startErr := core.StartInstance("json", serverJSON)
				if startErr != nil {
					t.Fatal(startErr)
				}
				defer server.Close()
			}
			mapping := forwardTCP(t, fmt.Sprintf("127.0.0.1:%d", serverPort))
			endpoint["port"] = mapping.Addr().(*net.TCPAddr).Port
			stream := out["streamSettings"].(map[string]any)
			stream["security"] = tc.security
			clientExtra := extra
			if strings.HasSuffix(tc.mode, "-legacy-padding") {
				clientExtra = map[string]any{"noSSEHeader": true}
			}
			stream["xhttpSettings"] = map[string]any{"host": "cdn.example", "path": "/transport/", "mode": strings.TrimSuffix(tc.mode, "-legacy-padding"), "extra": clientExtra}
			if tc.security == "reality" {
				delete(stream, "tlsSettings")
				stream["realitySettings"] = map[string]any{"serverName": "localhost", "fingerprint": "chrome", "password": base64.RawURLEncoding.EncodeToString(serverKey.PublicKey().Bytes()), "shortId": "1234"}
			} else if tc.security == "tls" {
				stream["tlsSettings"] = map[string]any{"serverName": "localhost", "allowInsecure": false, "certificates": []any{map[string]any{"usage": "verify", "certificate": strings.Split(certPEM, "\n")}}}
			} else {
				delete(stream, "tlsSettings")
			}
			socksPort := reservePort(t)
			clientConfig["inbounds"].([]any)[0].(map[string]any)["port"] = socksPort
			configPath := filepath.Join(t.TempDir(), "client.json")
			clientJSON, _ := json.Marshal(clientConfig)
			if err = os.WriteFile(configPath, clientJSON, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-config", configPath)
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cmd.Process.Kill(); cmd.Wait() }()
			socksAddress := fmt.Sprintf("127.0.0.1:%d", socksPort)
			for deadline := time.Now().Add(5 * time.Second); ; {
				probe, dialErr := net.DialTimeout("tcp", socksAddress, 100*time.Millisecond)
				if dialErr == nil {
					probe.Close()
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("helper SOCKS listener did not start")
				}
				time.Sleep(20 * time.Millisecond)
			}
			dialer, err := proxy.SOCKS5("tcp", socksAddress, nil, proxy.Direct)
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
			}}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
			response, err := client.Get(destination.URL)
			if strings.HasSuffix(tc.mode, "-legacy-padding") {
				if response != nil {
					response.Body.Close()
				}
				if err == nil {
					t.Fatal("server unexpectedly accepted incompatible padding settings")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != 200 || !bytes.Equal(body, payload) {
				t.Fatalf("tunnel payload mismatch: status=%d bytes=%d error=%v", response.StatusCode, len(body), err)
			}
		})
	}
}
