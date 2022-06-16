/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"context"
	"fmt"
	"strings"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	certmanclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DNSValidator int

const (
	DNSValidatorRoute53  DNSValidator = iota
	DefaultCertificateNS string       = "cert-manager"
)

type CertProvider string

// certManager is a certificate provider.
type certManager struct {
	dnsValidationProvider DNSValidator
	certClient            certmanclient.CertmanagerV1Interface
	k8sClient             kubernetes.Interface
	certProvider          CertProvider
	LEConfig              LEConfig
	Region                string
	certificateNS         string
	validDomains          []string
}

var _ Provider = &certManager{}

type LEConfig struct {
	Email string
}

type CertManagerConfig struct {
	DNSValidator DNSValidator
	CertClient   certmanclient.CertmanagerV1Interface

	CertProvider CertProvider
	LEConfig     *LEConfig
	Region       string
	// client targeting the control cluster
	K8sClient kubernetes.Interface
	// namespace in the control cluster where we create certificates
	CertificateNS string
	// set of domains we allow certs to be created for
	ValidDomains []string
}

func NewCertManager(c CertManagerConfig) (*certManager, error) {
	cm := &certManager{
		dnsValidationProvider: c.DNSValidator,
		certClient:            c.CertClient,
		k8sClient:             c.K8sClient,
		certProvider:          c.CertProvider,
		Region:                c.Region,
		validDomains:          c.ValidDomains,
		certificateNS:         c.CertificateNS,
	}

	return cm, nil
}

func (cm *certManager) IssuerID() string {
	return string(cm.certProvider)
}

func (cm *certManager) Domains() []string {
	return cm.validDomains
}

// Initialize will configure the issuer and aws access secret.
// TODO this should probably be in a controller for a GLBC CRD, or externalized in Issuer resources altogether
func (cm *certManager) Initialize(ctx context.Context) error {
	_, err := cm.certClient.Issuers(cm.certificateNS).Get(ctx, cm.IssuerID(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Issuer not %s found", cm.IssuerID())
		}
		return err
	}
	return nil
}

func (cm *certManager) Create(ctx context.Context, cr CertificateRequest) error {
	if !isValidDomain(cr.Host(), cm.validDomains) {
		return fmt.Errorf("cannot create certificate for host %s invalid domain", cr.Host())
	}
	cert := cm.certificate(cr)
	_, err := cm.certClient.Certificates(cm.certificateNS).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// TODO: Move to Certificate informer add handler
	CertificateRequestCount.WithLabelValues(cm.IssuerID()).Inc()
	return nil
}

func (cm *certManager) Delete(ctx context.Context, cr CertificateRequest) error {
	// delete the certificate and delete the secrets
	certNotFound := false

	if err := cm.certClient.Certificates(cm.certificateNS).Delete(ctx, cr.Name(), metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		certNotFound = true
	}
	if err := cm.k8sClient.CoreV1().Secrets(cm.certificateNS).Delete(ctx, cr.Name(), metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if !certNotFound {
			// The Secret does not exist, which indicates the TLS certificate request is still pending,
			// so we must account for decreasing the number of pending requests.
			// TODO: Move to Certificate informer delete handler
			CertificateRequestCount.WithLabelValues(cm.IssuerID()).Dec()
		}
	}
	return nil
}

func (cm *certManager) certificate(cr CertificateRequest) *certman.Certificate {
	annotations := cr.Annotations()
	annotations[tlsIssuerAnnotation] = cm.IssuerID()
	return &certman.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name(),
			Namespace: cm.certificateNS,
		},
		Spec: certman.CertificateSpec{
			SecretName: cr.Name(),
			SecretTemplate: &certman.CertificateSecretTemplate{
				Labels:      cr.Labels(),
				Annotations: annotations,
			},
			// TODO Some of the below should be pulled out into a CRD
			Duration: &metav1.Duration{
				Duration: time.Hour * 24 * 90, // cert lasts for 90 days
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Hour * 24 * 15, // cert is renewed 15 days before hand
			},
			PrivateKey: &certman.CertificatePrivateKey{
				Algorithm: certman.RSAKeyAlgorithm,
				Encoding:  certman.PKCS1,
				Size:      2048,
			},
			Usages:   certman.DefaultKeyUsages(),
			DNSNames: []string{cr.Host()},
			IssuerRef: cmmeta.ObjectReference{
				Group: "cert-manager.io",
				Kind:  "Issuer",
				Name:  string(cm.certProvider),
			},
		},
	}
}

func isValidDomain(host string, allowed []string) bool {
	for _, v := range allowed {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}
