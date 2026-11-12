//go:build with_utls

package tls

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	utls "github.com/metacubex/utls"
	"github.com/sagernet/sing-box/option"
	"golang.org/x/crypto/hkdf"
)

// Inspect the real serialized handshake and authenticate it as the server does.
// This catches stripping the hybrid share, wrong share order, and mismatched
// X25519 keys in the REALITY session-id authentication.
func TestRealityChromeWireCompatibility(t *testing.T) {
	serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config, err := NewRealityClient(context.Background(), "localhost", option.OutboundTLSOptions{
		ServerName: "localhost",
		UTLS:       &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
		Reality: &option.OutboundRealityOptions{
			Enabled: true, PublicKey: base64.RawURLEncoding.EncodeToString(serverKey.PublicKey().Bytes()),
			ShortID: "1234567890abcdef",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	deadline := time.Now().Add(5 * time.Second)
	client.SetDeadline(deadline)
	server.SetDeadline(deadline)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// The receiver closes after ClientHello; no handshake success is assumed.
		config.ClientHandshake(context.Background(), client)
	}()
	var header [5]byte
	if _, err = io.ReadFull(server, header[:]); err != nil {
		t.Fatal(err)
	}
	if header[0] != 22 {
		t.Fatalf("expected TLS handshake, got record %d", header[0])
	}
	raw := make([]byte, binary.BigEndian.Uint16(header[3:]))
	if _, err = io.ReadFull(server, raw); err != nil {
		t.Fatal(err)
	}
	server.Close()
	<-done
	hello := utls.UnmarshalClientHello(raw)
	if hello == nil {
		t.Fatal("invalid ClientHello")
	}
	var hybrid, plain []byte
	var hybridIndex, plainIndex = -1, -1
	for i, share := range hello.KeyShares {
		switch share.Group {
		case utls.X25519MLKEM768:
			hybrid, hybridIndex = share.Data, i
		case utls.X25519:
			plain, plainIndex = share.Data, i
		}
	}
	if len(hybrid) != 1216 || len(plain) != 32 || hybridIndex >= plainIndex {
		t.Fatalf("incompatible key shares: hybrid=%d at %d, plain=%d at %d", len(hybrid), hybridIndex, len(plain), plainIndex)
	}
	var hasHybridGroup bool
	for _, group := range hello.SupportedCurves {
		hasHybridGroup = hasHybridGroup || group == utls.X25519MLKEM768
	}
	if !hasHybridGroup {
		t.Fatal("missing hybrid group")
	}
	peer, err := ecdh.X25519().NewPublicKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	authKey, err := serverKey.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = hkdf.New(sha256.New, authKey, hello.Random[:20], []byte("REALITY")).Read(authKey); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(authKey)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	associated := append([]byte(nil), raw...)
	clear(associated[39:71])
	session, err := aead.Open(nil, hello.Random[20:], hello.SessionId, associated)
	if err != nil {
		t.Fatalf("server cannot authenticate session: %v", err)
	}
	if !bytes.Equal(session[8:], []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}) {
		t.Fatalf("unexpected short id: %x", session[8:])
	}
}

func TestRealityIncompleteHybridPair(t *testing.T) {
	for _, tc := range []struct {
		name         string
		group, share bool
	}{
		{"legacy", false, false},
		{"group only", true, false},
		{"share only", false, true},
		{"complete", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curves := &utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519}}
			shares := &utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}}
			if tc.group {
				curves.Curves = append(curves.Curves, utls.X25519MLKEM768)
			}
			if tc.share {
				shares.KeyShares = append(shares.KeyShares, utls.KeyShare{Group: utls.X25519MLKEM768})
			}
			prepareRealityKeyShares([]utls.TLSExtension{curves, shares})
			want := 1
			if tc.group && tc.share {
				want = 2
			}
			if len(curves.Curves) != want || len(shares.KeyShares) != want {
				t.Fatalf("inconsistent group/share pair: %v, %v", curves.Curves, shares.KeyShares)
			}
		})
	}
}
