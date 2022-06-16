package main

import (
	"fmt"
	"os"

	cmacme "github.com/jetstack/cert-manager/pkg/apis/acme/v1"
	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	leProdAPI    string = "https://acme-v02.api.letsencrypt.org/directory"
	leStagingAPI string = "https://acme-staging-v02.api.letsencrypt.org/directory"

	awsSecretName      string = "route53-credentials"
	envAwsAccessKeyID  string = "AWS_ACCESS_KEY_ID"
	envAwsAccessSecret string = "AWS_SECRET_ACCESS_KEY"
	envAwsZoneID       string = "AWS_DNS_PUBLIC_ZONE_ID"

	envLEEmail string = "HCG_LE_EMAIL"

	CertProviderLEStaging = "le-staging"
	CertProviderLEProd    = "le-production"
)

type LetsEncryptIssuer struct {
	tlsProvider    string
	server         string
	providerRegion string
}

var _ Issuer = &LetsEncryptIssuer{}

func NewLetsEncryptIssuer(tlsProvider string, providerRegion string) *LetsEncryptIssuer {
	server := leStagingAPI
	if tlsProvider == string(CertProviderLEProd) {
		server = leProdAPI
	}

	return &LetsEncryptIssuer{
		tlsProvider:    tlsProvider,
		server:         server,
		providerRegion: providerRegion,
	}
}

func (issuer *LetsEncryptIssuer) GetSecret() (*corev1.Secret, error) {
	if os.Getenv(envAwsAccessKeyID) == "" || os.Getenv(envAwsAccessSecret) == "" || os.Getenv(envAwsZoneID) == "" {
		return nil, fmt.Errorf(fmt.Sprintf("certmanager is missing envars for aws %s %s %s", envAwsAccessKeyID, envAwsAccessSecret, envAwsZoneID))
	}

	if os.Getenv(envLEEmail) == "" {
		return nil, fmt.Errorf("certmanager: missing env var %s", envLEEmail)
	}

	accessKeyID := os.Getenv(envAwsAccessKeyID)
	accessSecret := os.Getenv(envAwsAccessSecret)
	zoneID := os.Getenv(envAwsZoneID)

	data := make(map[string][]byte)

	data[envAwsAccessKeyID] = []byte(accessKeyID)
	data[envAwsAccessSecret] = []byte(accessSecret)
	data[envAwsZoneID] = []byte(zoneID)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: awsSecretName,
		},
		Data: data,
	}, nil
}

func (issuer *LetsEncryptIssuer) GetIssuer() *certman.Issuer {
	return &certman.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultCertificateNS,
			Name:      issuer.tlsProvider,
		},
		Spec: certman.IssuerSpec{
			IssuerConfig: certman.IssuerConfig{
				ACME: &cmacme.ACMEIssuer{
					Email:  os.Getenv(envLEEmail),
					Server: issuer.server,
					PrivateKey: cmmeta.SecretKeySelector{
						LocalObjectReference: cmmeta.LocalObjectReference{
							Name: issuer.tlsProvider,
						},
					},
					Solvers: []cmacme.ACMEChallengeSolver{
						{
							DNS01: &cmacme.ACMEChallengeSolverDNS01{
								Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{
									AccessKeyID:  os.Getenv(envAwsAccessKeyID),
									HostedZoneID: os.Getenv(envAwsZoneID),
									Region:       issuer.providerRegion,
									SecretAccessKey: cmmeta.SecretKeySelector{
										LocalObjectReference: cmmeta.LocalObjectReference{
											Name: awsSecretName,
										},
										Key: envAwsAccessSecret,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (issuer *LetsEncryptIssuer) GetTLSProvider() string {
	return issuer.tlsProvider
}
