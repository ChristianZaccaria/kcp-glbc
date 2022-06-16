package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	certmanclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	"github.com/kuadrant/kcp-glbc/pkg/util/env"
	genericapiserver "k8s.io/apiserver/pkg/server"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultCertificateNS string = "cert-manager"
	caSecretName         string = "glbc-ca"
)

type Issuer interface {
	GetSecret() (*corev1.Secret, error)
	GetIssuer() *certman.Issuer
	GetTLSProvider() string
}

func newIssuer(tlsProvider string, awsRegion string) Issuer {
	var issuer Issuer
	switch tlsProvider {
	case CertProviderCA:
		issuer = NewCAIssuer()
	case CertProviderLEStaging, CertProviderLEProd:
		issuer = NewLetsEncryptIssuer(tlsProvider, awsRegion)
	default:
		log.Fatalln(fmt.Errorf("unsupported TLS certificate issuer: %s", issuer.GetTLSProvider()))
	}
	return issuer
}

func main() {

	// Control cluster client options
	var glbcKubeconfig string
	var tlsProvider string = ""
	var awsRegion string = ""

	flag.StringVar(&glbcKubeconfig, "glbc-kubeconfig", "", "Path to GLBC kubeconfig")
	flag.StringVar(&tlsProvider, "glbc-tls-provider", env.GetEnvString("GLBC_TLS_PROVIDER", "glbc-ca"), "The TLS certificate issuer, one of [glbc-ca, le-staging, le-production]")
	flag.StringVar(&awsRegion, "region", env.GetEnvString("AWS_REGION", "eu-central-1"), "the region we should target with AWS clients")
	flag.Parse()

	issuer := newIssuer(tlsProvider, awsRegion)
	glbcClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: glbcKubeconfig},
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		log.Fatalln(fmt.Errorf("Failed to create K8S config %w", err))
	}

	certManagerClient := certmanclient.NewForConfigOrDie(glbcClientConfig)

	// glbcKubeClient targets the control cluster (this is the cluster where glbc is deployed).
	// This is not a KCP cluster.
	glbcKubeClient, err := kubernetes.NewForConfig(glbcClientConfig)
	if err != nil {
		log.Fatalln(fmt.Errorf("Failed to create K8S core client %w", err))
	}

	ctx := genericapiserver.SetupSignalContext()
	if err := create(ctx, certManagerClient, glbcKubeClient, issuer); err != nil {
		log.Fatalln(fmt.Errorf("failed to create issuer for : %s %w", tlsProvider, err))
	}

	log.Printf("Issuer %s successfully created ", tlsProvider)
}

func create(ctx context.Context, certManegerClient certmanclient.CertmanagerV1Interface, k8sClient kubernetes.Interface, issuerObject Issuer) error {

	issuerSecret, err := issuerObject.GetSecret()
	if err != nil {
		return err
	}

	secretClientInterface := k8sClient.CoreV1().Secrets(defaultCertificateNS)
	secret, err := secretClientInterface.Get(ctx, issuerSecret.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		_, err = secretClientInterface.Create(ctx, issuerSecret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		issuerSecret.SetResourceVersion(secret.ResourceVersion)
		_, err = secretClientInterface.Update(ctx, issuerSecret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	issuerClientInterface := certManegerClient.Issuers(defaultCertificateNS)
	issuer := issuerObject.GetIssuer()
	issuerInstance, err := issuerClientInterface.Get(ctx, issuer.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		_, err = issuerClientInterface.Create(ctx, issuer, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		issuer.SetResourceVersion(issuerInstance.GetResourceVersion())
		_, err = issuerClientInterface.Update(ctx, issuer, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
