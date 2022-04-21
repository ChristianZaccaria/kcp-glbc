package tls

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	secretsv1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kuadrant/kcp-glbc/pkg/reconciler"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
)

const (
	controllerName      = "kcp-glbc-secrets"
	tlsIssuerAnnotation = "kuadrant.dev/tls-issuer"
)

type ControllerConfig struct {
	// this is targeting our own kube api not KCP
	GlbcKubeClient        kubernetes.Interface
	SharedInformerFactory informers.SharedInformerFactory
	KcpClient             kubernetes.ClusterInterface
}

type Controller struct {
	reconciler.Controller
	glbcKubeClient        kubernetes.Interface
	lister                secretsv1lister.SecretLister
	indexer               cache.Indexer
	sharedInformerFactory informers.SharedInformerFactory
	kcpClient             kubernetes.ClusterInterface
}

func NewController(config *ControllerConfig) (*Controller, error) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c := &Controller{
		Controller: reconciler.Controller{
			Name:  controllerName,
			Queue: queue,
		},
		glbcKubeClient:        config.GlbcKubeClient,
		kcpClient:             config.KcpClient,
		sharedInformerFactory: config.SharedInformerFactory,
	}
	c.Process = c.process

	c.sharedInformerFactory.Core().V1().Secrets().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if issuer, ok := secret.Annotations[tlsIssuerAnnotation]; ok {
				tlsCertificateSecretCount.WithLabelValues(issuer).Inc()
				// The certificate request has successfully completed
				tlsCertificateRequestTotal.WithLabelValues(issuer, resultLabelSucceeded).Inc()
				// The certificate request has successfully completed so there is one less pending request
				tls.CertificateRequestCount.WithLabelValues(issuer).Dec()
			}
			c.Enqueue(obj)
		},
		UpdateFunc: func(_, obj interface{}) {
			c.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if issuer, ok := secret.Annotations[tlsIssuerAnnotation]; ok {
				tlsCertificateSecretCount.WithLabelValues(issuer).Dec()
			}
			c.Enqueue(obj)
		},
	})

	c.indexer = c.sharedInformerFactory.Core().V1().Secrets().Informer().GetIndexer()
	c.lister = c.sharedInformerFactory.Core().V1().Secrets().Lister()

	return c, nil
}

func (c *Controller) process(ctx context.Context, key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		klog.Infof("Object with key %q was deleted", key)
		return nil
	}
	current := obj.(*v1.Secret)

	previous := current.DeepCopy()
	if err := c.reconcile(ctx, current); err != nil {
		return err
	}
	// If the object being reconciled changed as a result, update it.
	if !equality.Semantic.DeepEqual(previous, current) {
		_, err := c.glbcKubeClient.CoreV1().Secrets(current.Namespace).Update(ctx, current, metav1.UpdateOptions{})
		return err
	}
	return nil
}
