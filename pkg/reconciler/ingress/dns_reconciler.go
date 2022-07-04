package ingress

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/cluster"
	"github.com/kuadrant/kcp-glbc/pkg/dns/aws"
	"github.com/kuadrant/kcp-glbc/pkg/net"
	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/util/slice"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type dnsReconciler struct {
	deleteDNS        func(ctx context.Context, ingress *networkingv1.Ingress) error
	getDNS           func(ctx context.Context, ingress *networkingv1.Ingress) (*v1.DNSRecord, error)
	createDNS        func(ctx context.Context, dns *v1.DNSRecord) (*v1.DNSRecord, error)
	patchDNS         func(ctx context.Context, dns *v1.DNSRecord, data []byte) (*v1.DNSRecord, error)
	watchHost        func(ctx context.Context, key interface{}, host string) bool
	forgetHost       func(key interface{}, host string)
	listHostWatchers func(key interface{}) []net.RecordWatcher
	DNSLookup        func(ctx context.Context, host string) ([]net.HostAddress, error)
}

func (r *dnsReconciler) reconcile(ctx context.Context, ingress *networkingv1.Ingress) (reconcileStatus, error) {
	if ingress.DeletionTimestamp != nil && !ingress.DeletionTimestamp.IsZero() {
		// delete DNSRecord
		if err := r.deleteDNS(ctx, ingress); err != nil && !errors.IsNotFound(err) {
			return reconcileStatusStop, err
		}
		return reconcileStatusContinue, nil
	}

	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		key := ingressKey(ingress)
		var activeHosts []string
		// Start watching for address changes in the LBs hostnames
		for _, lbs := range ingress.Status.LoadBalancer.Ingress {
			if lbs.Hostname != "" {
				r.watchHost(ctx, key, lbs.Hostname)
				activeHosts = append(activeHosts, lbs.Hostname)
			}
		}

		hostRecordWatchers := r.listHostWatchers(key)
		for _, watcher := range hostRecordWatchers {
			if !slice.ContainsString(activeHosts, watcher.Host) {
				r.forgetHost(key, watcher.Host)
			}
		}

		// Attempt to retrieve the existing DNSRecord for this Ingress
		existing, err := r.getDNS(ctx, ingress)
		// If it doesn't exist, create it

		if err != nil && errors.IsNotFound(err) {
			// Create the DNSRecord object
			record := &v1.DNSRecord{}
			if err := r.setDnsRecordFromIngress(ctx, ingress, record); err != nil {
				return reconcileStatusStop, err
			}
			// Create the resource in the cluster
			existing, err = r.createDNS(ctx, record)
			if err != nil {
				return reconcileStatusStop, err
			}

			// metric to observe the ingress admission time
			ingressObjectTimeToAdmission.
				Observe(existing.CreationTimestamp.Time.Sub(ingress.CreationTimestamp.Time).Seconds())

		} else if err == nil {
			// If it does exist, update it
			if err := r.setDnsRecordFromIngress(ctx, ingress, existing); err != nil {
				return reconcileStatusStop, err
			}

			data, err := json.Marshal(existing)
			if err != nil {
				return reconcileStatusStop, err
			}
			if _, err = r.patchDNS(ctx, existing, data); err != nil {
				return reconcileStatusStop, err
			}
		} else {
			return reconcileStatusStop, err
		}
	}

	return reconcileStatusContinue, nil
}

func (r *dnsReconciler) setDnsRecordFromIngress(ctx context.Context, ingress *networkingv1.Ingress, dnsRecord *v1.DNSRecord) error {
	dnsRecord.TypeMeta = metav1.TypeMeta{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "DNSRecord",
	}
	dnsRecord.ObjectMeta = metav1.ObjectMeta{
		Name:        ingress.Name,
		Namespace:   ingress.Namespace,
		ClusterName: ingress.ClusterName,
	}

	metadata.CopyAnnotationsPredicate(ingress, dnsRecord, metadata.KeyPredicate(func(key string) bool {
		return strings.HasPrefix(key, cluster.ANNOTATION_HEALTH_CHECK_PREFIX)
	}))

	// Sets the Ingress as the owner reference
	dnsRecord.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         networkingv1.SchemeGroupVersion.String(),
			Kind:               "Ingress",
			Name:               ingress.Name,
			UID:                ingress.UID,
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		},
	})

	return r.setEndpointsFromIngress(ctx, ingress, dnsRecord)
}

func (r *dnsReconciler) setEndpointsFromIngress(ctx context.Context, ingress *networkingv1.Ingress, dnsRecord *v1.DNSRecord) error {
	targets, err := r.targetsFromIngressStatus(ctx, ingress.Status)
	if err != nil {
		return err
	}

	hostname := ingress.Annotations[cluster.ANNOTATION_HCG_HOST]

	// Build a map[Address]Endpoint with the current endpoints to assist
	// finding endpoints that match the targets
	currentEndpoints := make(map[string]*v1.Endpoint, len(dnsRecord.Spec.Endpoints))
	for _, endpoint := range dnsRecord.Spec.Endpoints {
		address, ok := endpoint.GetAddress()
		if !ok {
			continue
		}

		currentEndpoints[address] = endpoint
	}

	var newEndpoints []*v1.Endpoint

	for _, ingressTargets := range targets {
		for _, target := range ingressTargets {
			var endpoint *v1.Endpoint
			ok := false

			// If the endpoint for this target does not exist, add a new one
			if endpoint, ok = currentEndpoints[target]; !ok {
				endpoint = &v1.Endpoint{
					SetIdentifier: target,
				}
			}

			newEndpoints = append(newEndpoints, endpoint)

			// Update the endpoint fields
			endpoint.DNSName = hostname
			endpoint.RecordType = "A"
			endpoint.Targets = []string{target}
			endpoint.RecordTTL = 60
			endpoint.SetProviderSpecific(aws.ProviderSpecificWeight, awsEndpointWeight(len(ingressTargets)))
		}
	}

	dnsRecord.Spec.Endpoints = newEndpoints
	return nil
}

// targetsFromIngressStatus returns a map of all the IPs associated with a single ingress(cluster)
func (r *dnsReconciler) targetsFromIngressStatus(ctx context.Context, status networkingv1.IngressStatus) (map[string][]string, error) {
	var targets = make(map[string][]string, len(status.LoadBalancer.Ingress))

	for _, lb := range status.LoadBalancer.Ingress {
		if lb.IP != "" {
			targets[lb.IP] = []string{lb.IP}
		}
		if lb.Hostname != "" {
			ips, err := r.DNSLookup(ctx, lb.Hostname)
			if err != nil {
				return nil, err
			}
			targets[lb.Hostname] = []string{}
			for _, ip := range ips {
				targets[lb.Hostname] = append(targets[lb.Hostname], ip.IP.String())
			}
		}
	}
	return targets, nil
}

// awsEndpointWeight returns the weight value for a single AWS record in a set of records where the traffic is split
// evenly between a number of clusters/ingresses, each splitting traffic evenly to a number of IPs (numIPs)
//
// Divides the number of IPs by a known weight allowance for a cluster/ingress, note that this means:
// * Will always return 1 after a certain number of ips is reached, 60 in the current case (maxWeight / 2)
// * Will return values that don't add up to the total maxWeight when the number of ingresses is not divisible by numIPs
//
// The aws weight value must be an integer between 0 and 255.
// https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-weighted.html#rrsets-values-weighted-weight
func awsEndpointWeight(numIPs int) string {
	maxWeight := 120
	if numIPs > maxWeight {
		numIPs = maxWeight
	}
	return strconv.Itoa(maxWeight / numIPs)
}
