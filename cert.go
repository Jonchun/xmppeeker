package main

// Mostly taken from
// https://gist.github.com/samuel/8b500ddd3f6118d052b5e6bc16bc4c09

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func generateSelfSignedCert() (certPem []byte, keyPem []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"XMPPeeker"},
		},
		NotBefore: time.Now().Add(-1 * 24 * time.Hour),
		NotAfter:  time.Now().Add(10 * 365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return nil, nil, err
	}
	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certPem = append(out.Bytes()[:0:0], out.Bytes()...)
	out.Reset()
	pem.Encode(out, pemBlockForKey(priv))
	keyPem = append(out.Bytes()[:0:0], out.Bytes()...)
	return certPem, keyPem, nil
}

func generateAndSaveSelfSignedCert(logger *zap.SugaredLogger) (cert tls.Certificate, err error) {

	certPem, keyPem, err := generateSelfSignedCert()
	if err != nil {
		logger.Warnw("failed to generate self-signed certificate",
			"reason", err.Error(),
		)
		os.Exit(ExitBadConfig)
	}
	cert, err = tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return
	}

	// Save autogen certs to disk
	f, e := os.OpenFile(filepath.Join(AppRoot, DefaultCertificatePath, DefaultCertificate), os.O_RDWR|os.O_CREATE, 0755)
	if e != nil {
		logger.Errorw("failed to open certificate file",
			"reason", err.Error(),
		)
	} else {
		if _, err := f.Write(certPem); err != nil {
			logger.Errorw("failed to save generated self-signed certificate",
				"reason", err.Error(),
			)
		} else {
			logger.Infow("generated a self-signed certificate for use.",
				"file", DefaultCertificate,
			)
		}
	}
	f, err = os.OpenFile(filepath.Join(AppRoot, DefaultCertificatePath, DefaultCertificateKey), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		logger.Errorw("failed to open key file",
			"reason", err.Error(),
		)
	} else {
		if _, err := f.Write(keyPem); err != nil {
			logger.Errorw("failed to save generated self-signed certificate key",
				"reason", err.Error(),
			)
		} else {
			logger.Infow("generated a self-signed certificate key for use.",
				"file", DefaultCertificateKey,
			)
		}
	}
	return
}
