package ingress

import (
	"context"
	"fmt"

	"github.com/kuadrant/kcp-glbc/pkg/cluster"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type certificateReconciler struct {
	createCertificate func(ctx context.Context, mapper tls.CertificateRequest) error
	deleteCertificate func(ctx context.Context, mapper tls.CertificateRequest) error
	patchIngress      func(ctx context.Context, ingress *networkingv1.Ingress, data []byte) (*networkingv1.Ingress, error)
}

func (r *certificateReconciler) reconcile(ctx context.Context, ingress *networkingv1.Ingress) (reconcileStatus, error) {

	controlClusterContext, err := cluster.NewControlObjectMapper(ingress)
	if err != nil {
		return reconcileStatusStop, err
	}
	if ingress.DeletionTimestamp != nil && !ingress.DeletionTimestamp.IsZero() {
		if err := r.deleteCertificate(ctx, controlClusterContext); err != nil {
			return reconcileStatusStop, err
		}
		return reconcileStatusContinue, nil
	}

	err = r.createCertificate(ctx, controlClusterContext)
	if err != nil && !errors.IsAlreadyExists(err) {
		return reconcileStatusStop, err
	}
	patch := fmt.Sprintf(`{"spec":{"tls":[{"hosts":[%q],"secretName":%q}]}}`, controlClusterContext.Host(), controlClusterContext.Name())
	if _, err := r.patchIngress(ctx, ingress, []byte(patch)); err != nil {
		return reconcileStatusStop, err
	}

	return reconcileStatusContinue, nil
}

func removeHostsFromTLS(hostsToRemove []string, ingress *networkingv1.Ingress) {
	for _, host := range hostsToRemove {
		for i, tls := range ingress.Spec.TLS {
			hosts := tls.Hosts
			for j, ingressHost := range tls.Hosts {
				if ingressHost == host {
					hosts = append(hosts[:j], hosts[j+1:]...)
				}
			}
			// if there are no hosts remaining remove the entry for TLS
			if len(hosts) == 0 {
				ingress.Spec.TLS[i] = networkingv1.IngressTLS{}
			} else {
				ingress.Spec.TLS[i].Hosts = hosts
			}
		}
	}
}
