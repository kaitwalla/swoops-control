package certgen

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// GenerateClientCertificate generates a client certificate and private key for mTLS
// Returns PEM-encoded cert and key
func GenerateClientCertificate(caCertPEM, caKeyPEM []byte, commonName string) (certPEM, keyPEM []byte, err error) {
	// Parse CA certificate
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Parse CA private key (try both RSA and ECDSA)
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key PEM")
	}

	var caKey crypto.Signer
	// Try parsing as PKCS8 first (most common)
	if key, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			caKey = k
		case *ecdsa.PrivateKey:
			caKey = k
		default:
			return nil, nil, fmt.Errorf("unsupported CA private key type: %T", key)
		}
	} else if key, err := x509.ParseECPrivateKey(caKeyBlock.Bytes); err == nil {
		caKey = key
	} else if key, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes); err == nil {
		caKey = key
	} else {
		return nil, nil, fmt.Errorf("failed to parse CA private key")
	}

	// Generate client private key (ECDSA P256)
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate client private key: %w", err)
	}

	// Create client certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	return certPEM, keyPEM, nil
}

// GenerateClientCertificateFromFiles generates a client certificate using CA files from disk
func GenerateClientCertificateFromFiles(caCertPath, caKeyPath, commonName string) (certPEM, keyPEM []byte, err error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA key: %w", err)
	}

	return GenerateClientCertificate(caCertPEM, caKeyPEM, commonName)
}
