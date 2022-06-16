package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	mathrand "math/rand"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CertProviderCA = "glbc-ca"
)

type CAIssuer struct {
	tlsProvider string
}

var _ Issuer = &CAIssuer{}

func NewCAIssuer() *CAIssuer {
	return &CAIssuer{
		tlsProvider: CertProviderCA,
	}
}

func (issuer *CAIssuer) GetSecret() (*corev1.Secret, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(mathrand.Int63()),
		Subject: pkix.Name{
			Organization: []string{"Kuadrant"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, err
	}

	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	privateKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: caSecretName,
		}, Data: map[string][]byte{
			corev1.TLSCertKey:       certPem,
			corev1.TLSPrivateKeyKey: privateKeyPem,
		}, Type: corev1.SecretTypeTLS,
	}, nil
}

func (issuer *CAIssuer) GetIssuer() *certman.Issuer {
	return &certman.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultCertificateNS,
			Name:      issuer.tlsProvider,
		},
		Spec: certman.IssuerSpec{
			IssuerConfig: certman.IssuerConfig{
				CA: &certman.CAIssuer{
					SecretName: issuer.tlsProvider,
				},
			},
		},
	}
}

func (issuer *CAIssuer) GetTLSProvider() string {
	return issuer.tlsProvider
}
