package ingress

import (
	"context"
	"fmt"

	"github.com/kuadrant/kcp-glbc/pkg/cluster"
	"github.com/rs/xid"
	networkingv1 "k8s.io/api/networking/v1"
)

type hostReconciler struct {
	managedDomain string
	updateIngress func(ctx context.Context, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)
}

func (r *hostReconciler) reconcile(ctx context.Context, ingress *networkingv1.Ingress) (reconcileStatus, error) {
	if ingress.Annotations == nil || ingress.Annotations[cluster.ANNOTATION_HCG_HOST] == "" {
		var err error
		// Let's assign it a global hostname if any
		generatedHost := fmt.Sprintf("%s.%s", xid.New(), r.managedDomain)
		if ingress.Annotations == nil {
			ingress.Annotations = map[string]string{}
		}
		ingress.Annotations[cluster.ANNOTATION_HCG_HOST] = generatedHost
		ingress, err = r.updateIngress(ctx, ingress)
		if err != nil {
			return reconcileStatusStop, err
		}
	}
	managedHost := ingress.Annotations[cluster.ANNOTATION_HCG_HOST]

	var customHosts []string
	for i, rule := range ingress.Spec.Rules {
		if rule.Host != managedHost {
			ingress.Spec.Rules[i].Host = managedHost
			customHosts = append(customHosts, rule.Host)
		}
	}

	// clean up replaced hosts from the tls list
	removeHostsFromTLS(customHosts, ingress)

	if len(customHosts) > 0 {
		ingress.Annotations[cluster.ANNOTATION_HCG_CUSTOM_HOST_REPLACED] = fmt.Sprintf(" replaced custom hosts %v to the glbc host due to custom host policy not being allowed",
			customHosts)
		if _, err := r.updateIngress(ctx, ingress); err != nil {
			return reconcileStatusStop, err
		}
	}

	return reconcileStatusContinue, nil
}
