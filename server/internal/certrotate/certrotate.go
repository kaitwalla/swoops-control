package certrotate

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
	"sync"
	"time"
)

// CertRotator monitors certificate files and automatically reloads them when they change
type CertRotator struct {
	certFile string
	keyFile  string
	caFile   string // Optional CA file for mTLS

	mu          sync.RWMutex
	certificate *tls.Certificate
	caCertPool  *x509.CertPool

	logger     *slog.Logger
	stopCh     chan struct{}
	checkInterval time.Duration
}

// NewCertRotator creates a new certificate rotator
func NewCertRotator(certFile, keyFile, caFile string, logger *slog.Logger) (*CertRotator, error) {
	cr := &CertRotator{
		certFile:      certFile,
		keyFile:       keyFile,
		caFile:        caFile,
		logger:        logger,
		stopCh:        make(chan struct{}),
		checkInterval: 5 * time.Minute, // Check every 5 minutes
	}

	// Load initial certificate
	if err := cr.loadCertificate(); err != nil {
		return nil, err
	}

	// Load CA if provided
	if caFile != "" {
		if err := cr.loadCA(); err != nil {
			return nil, err
		}
	}

	// Start monitoring
	go cr.monitor()

	return cr, nil
}

// loadCertificate loads the certificate from disk
func (cr *CertRotator) loadCertificate() error {
	cert, err := tls.LoadX509KeyPair(cr.certFile, cr.keyFile)
	if err != nil {
		return err
	}

	cr.mu.Lock()
	cr.certificate = &cert
	cr.mu.Unlock()

	cr.logger.Info("Certificate loaded",
		"cert_file", cr.certFile,
		"key_file", cr.keyFile,
	)

	return nil
}

// loadCA loads the CA certificate pool from disk
func (cr *CertRotator) loadCA() error {
	caCert, err := os.ReadFile(cr.caFile)
	if err != nil {
		return err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		cr.logger.Warn("Failed to parse CA certificate",
			"ca_file", cr.caFile,
		)
	}

	cr.mu.Lock()
	cr.caCertPool = certPool
	cr.mu.Unlock()

	cr.logger.Info("CA certificate loaded",
		"ca_file", cr.caFile,
	)

	return nil
}

// GetCertificate returns the current certificate (thread-safe)
func (cr *CertRotator) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.certificate, nil
}

// GetCertificateFunc returns a function suitable for tls.Config.GetCertificate
func (cr *CertRotator) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return cr.GetCertificate
}

// GetCACertPool returns the current CA certificate pool (thread-safe)
func (cr *CertRotator) GetCACertPool() *x509.CertPool {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.caCertPool
}

// monitor watches for certificate file changes and reloads them
func (cr *CertRotator) monitor() {
	ticker := time.NewTicker(cr.checkInterval)
	defer ticker.Stop()

	// Track last modification times
	lastCertMod := cr.getModTime(cr.certFile)
	lastKeyMod := cr.getModTime(cr.keyFile)
	var lastCAMod time.Time
	if cr.caFile != "" {
		lastCAMod = cr.getModTime(cr.caFile)
	}

	for {
		select {
		case <-ticker.C:
			certMod := cr.getModTime(cr.certFile)
			keyMod := cr.getModTime(cr.keyFile)

			// Check if cert or key files changed
			if !certMod.Equal(lastCertMod) || !keyMod.Equal(lastKeyMod) {
				cr.logger.Info("Certificate files changed, reloading...",
					"cert_file", cr.certFile,
					"key_file", cr.keyFile,
				)

				if err := cr.loadCertificate(); err != nil {
					cr.logger.Error("Failed to reload certificate",
						"error", err,
						"cert_file", cr.certFile,
						"key_file", cr.keyFile,
					)
				} else {
					cr.logger.Info("Certificate reloaded successfully")
					lastCertMod = certMod
					lastKeyMod = keyMod
				}
			}

			// Check if CA file changed
			if cr.caFile != "" {
				caMod := cr.getModTime(cr.caFile)
				if !caMod.Equal(lastCAMod) {
					cr.logger.Info("CA certificate file changed, reloading...",
						"ca_file", cr.caFile,
					)

					if err := cr.loadCA(); err != nil {
						cr.logger.Error("Failed to reload CA certificate",
							"error", err,
							"ca_file", cr.caFile,
						)
					} else {
						cr.logger.Info("CA certificate reloaded successfully")
						lastCAMod = caMod
					}
				}
			}

		case <-cr.stopCh:
			return
		}
	}
}

// getModTime returns the modification time of a file
func (cr *CertRotator) getModTime(filename string) time.Time {
	info, err := os.Stat(filename)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Stop stops the certificate monitoring
func (cr *CertRotator) Stop() {
	close(cr.stopCh)
}

// SetCheckInterval sets the certificate check interval
func (cr *CertRotator) SetCheckInterval(interval time.Duration) {
	cr.checkInterval = interval
}
