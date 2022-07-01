package ingress

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kuadrant/kcp-glbc/pkg/cluster"
	networkingv1 "k8s.io/api/networking/v1"
)

type hostResult struct {
	Status  reconcileStatus
	Err     error
	Ingress *networkingv1.Ingress
}

func TestReconcileHost(t *testing.T) {
	ingress := func(rules []networkingv1.IngressRule) *networkingv1.Ingress {
		return &networkingv1.Ingress{
			Spec: networkingv1.IngressSpec{
				Rules: rules,
			},
		}
	}

	var buildResult = func(r reconciler, i *networkingv1.Ingress) hostResult {
		status, err := r.reconcile(context.TODO(), i)
		return hostResult{
			Status:  status,
			Err:     err,
			Ingress: i,
		}
	}

	var mangedDomain = "test.com"
	cases := []struct {
		Name     string
		Ingress  *networkingv1.Ingress
		Validate func(hr hostResult) error
	}{
		{
			Name: "test managed host generated for empty host field",
			Ingress: ingress([]networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{},
				},
			}, {
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{},
				},
			}}),
			Validate: func(hr hostResult) error {
				if hr.Status != reconcileStatusContinue {
					return fmt.Errorf("expected status to be continue but got stop")
				}
				if hr.Err != nil {
					return fmt.Errorf("unexpected error from reconcile : %s", hr.Err)
				}
				v, ok := hr.Ingress.Annotations[cluster.ANNOTATION_HCG_HOST]
				if !ok {
					return fmt.Errorf("expected annotation %s to be set", cluster.ANNOTATION_HCG_HOST)
				}
				for _, r := range hr.Ingress.Spec.Rules {
					if r.Host != v || !strings.HasSuffix(r.Host, mangedDomain) {
						return fmt.Errorf("generated host %s unexpected value ", r.Host)
					}
				}

				return nil
			},
		},
		{
			Name: "test custom host replaced with generated managed host",
			Ingress: ingress([]networkingv1.IngressRule{{
				Host: "api.example.com",
			}}),
			Validate: func(hr hostResult) error {
				if hr.Status != reconcileStatusContinue {
					return fmt.Errorf("expected status to be continue but got stop")
				}
				if hr.Err != nil {
					return fmt.Errorf("unexpected error from reconcile : %s", hr.Err)
				}
				if _, ok := hr.Ingress.Annotations[cluster.ANNOTATION_HCG_CUSTOM_HOST_REPLACED]; !ok {
					return fmt.Errorf("expected annotation %s to be set ", cluster.ANNOTATION_HCG_CUSTOM_HOST_REPLACED)
				}
				for _, r := range hr.Ingress.Spec.Rules {
					if !strings.HasSuffix(r.Host, mangedDomain) {
						return fmt.Errorf("expected all hosts to end with %s", mangedDomain)
					}
				}
				return nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			reconciler := &hostReconciler{
				managedDomain: mangedDomain,
				updateIngress: func(ctx context.Context, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
					return ingress, nil
				},
			}

			if err := tc.Validate(buildResult(reconciler, tc.Ingress)); err != nil {
				t.Fatalf("fail: %s", err)
			}

		})
	}
}
