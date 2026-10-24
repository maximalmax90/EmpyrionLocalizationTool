package store

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	tlsCertFile = "tls-cert.pem"
	tlsKeyFile  = "tls-key.pem"
)

func (s *Store) tlsCertPath() string { return filepath.Join(s.localConfigDir(), tlsCertFile) }
func (s *Store) tlsKeyPath() string  { return filepath.Join(s.localConfigDir(), tlsKeyFile) }

func (s *Store) EnsureSelfSignedCert() (certPath, keyPath string, err error) {
	certPath, keyPath = s.tlsCertPath(), s.tlsKeyPath()

	if certExists, err := fileExists(certPath); err != nil {
		return "", "", err
	} else if keyExists, err := fileExists(keyPath); err != nil {
		return "", "", err
	} else if certExists && keyExists {
		return certPath, keyPath, nil
	}

	if err := os.MkdirAll(s.localConfigDir(), 0o755); err != nil {
		return "", "", fmt.Errorf("store: creating %s: %w", localConfigDirName, err)
	}
	if err := generateSelfSignedCert(certPath, keyPath); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("store: generating TLS key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("store: generating certificate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "elt serve (self-signed)"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("store: creating certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("store: marshaling TLS key: %w", err)
	}

	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}

	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return fmt.Errorf("store: generated TLS cert/key failed to load: %w", err)
	}
	return nil
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("store: writing %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}
