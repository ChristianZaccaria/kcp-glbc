package ingress

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/kcp-dev/logicalcluster"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	utilserrors "k8s.io/apimachinery/pkg/util/errors"
)

type reconcileStatus int

const (
	manager                             = "kcp-ingress"
	reconcileStatusStop reconcileStatus = iota
	reconcileStatusContinue
)

type reconciler interface {
	reconcile(ctx context.Context, ingress *networkingv1.Ingress) (reconcileStatus, error)
}

func (c *Controller) reconcile(ctx context.Context, ingress *networkingv1.Ingress) error {
	reconcilers := []reconciler{
		&hostReconciler{
			updateIngress: c.updateIngress,
			managedDomain: c.domain,
		},
		&certificateReconciler{
			createCertificate: c.certProvider.Create,
			deleteCertificate: c.certProvider.Delete,
			patchIngress:      c.patchIngress,
		},
		&dnsReconciler{
			deleteDNS:        c.deleteDNS,
			DNSLookup:        c.hostResolver.LookupIPAddr,
			getDNS:           c.getDNS,
			createDNS:        c.createDNS,
			patchDNS:         c.patchDNS,
			watchHost:        c.hostsWatcher.StartWatching,
			forgetHost:       c.hostsWatcher.StopWatching,
			listHostWatchers: c.hostsWatcher.ListHostRecordWatchers,
		},
	}
	var errs []error

	for _, r := range reconcilers {
		status, err := r.reconcile(ctx, ingress)
		if err != nil {
			errs = append(errs, err)
		}
		if status == reconcileStatusStop {
			break
		}
	}

	return utilserrors.NewAggregate(errs)
}

func (c *Controller) patchIngress(ctx context.Context, ingress *networkingv1.Ingress, data []byte) (*networkingv1.Ingress, error) {
	return c.kubeClient.Cluster(logicalcluster.From(ingress)).NetworkingV1().Ingresses(ingress.Namespace).
		Patch(ctx, ingress.Name, types.MergePatchType, data, metav1.PatchOptions{FieldManager: manager})
}

func (c *Controller) patchDNS(ctx context.Context, dns *v1.DNSRecord, data []byte) (*v1.DNSRecord, error) {
	return c.dnsRecordClient.Cluster(logicalcluster.From(dns)).KuadrantV1().DNSRecords(dns.Namespace).Patch(ctx, dns.Name, types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: manager, Force: pointer.Bool(true)})
}

func (c *Controller) deleteDNS(ctx context.Context, ingress *networkingv1.Ingress) error {
	return c.dnsRecordClient.Cluster(logicalcluster.From(ingress)).KuadrantV1().DNSRecords(ingress.Namespace).Delete(ctx, ingress.Name, metav1.DeleteOptions{})
}

func (c *Controller) getDNS(ctx context.Context, ingress *networkingv1.Ingress) (*v1.DNSRecord, error) {
	return c.dnsRecordClient.Cluster(logicalcluster.From(ingress)).KuadrantV1().DNSRecords(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
}

func (c *Controller) createDNS(ctx context.Context, dnsRecord *v1.DNSRecord) (*v1.DNSRecord, error) {
	return c.dnsRecordClient.Cluster(logicalcluster.From(dnsRecord)).KuadrantV1().DNSRecords(dnsRecord.Namespace).Create(ctx, dnsRecord, metav1.CreateOptions{})
}

func ingressKey(ingress *networkingv1.Ingress) interface{} {
	key, _ := cache.MetaNamespaceKeyFunc(ingress)
	return cache.ExplicitKey(key)
}

func (c *Controller) updateIngress(ctx context.Context, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	i, err := c.kubeClient.Cluster(logicalcluster.From(ingress)).NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return i, nil
}
