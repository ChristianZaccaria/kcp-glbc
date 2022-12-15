package traffic

import (
	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_tmcEnabled(t *testing.T) {
	tests := []struct {
		name   string
		obj    metav1.Object
		expect bool
	}{
		{
			name: "metadata has the experimental.status.workload.kcp.dev/ annotation prefix",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"experimental.status.workload.kcp.dev/": "sync-target",
					},
				},
			},
			expect: true,
		},
		{
			name: "metadata does not have the experimental.status.workload.kcp.dev/ annotation prefix",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"test-fail": "sync-target",
					},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmcEnabled(tt.obj)
			if !got == tt.expect {
				t.Errorf("expected '%v' got '%v'", tt.expect, got)
			}
		})
	}
}

func Test_isDomainVerified(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		dvs    []v1.DomainVerification
		expect bool
	}{
		{
			name: "Domain that matches host and has been verified",
			host: "cz.hcpapps.net",
			dvs: []v1.DomainVerification{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cz.hcpapps.net",
					},
					Spec: v1.DomainVerificationSpec{
						Domain: "cz.hcpapps.net",
					},
					Status: v1.DomainVerificationStatus{
						Verified: true,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cz.false.net",
					},
					Spec: v1.DomainVerificationSpec{
						Domain: "cz.false.net",
					},
					Status: v1.DomainVerificationStatus{
						Verified: false,
					},
				},
			},
			expect: true,
		},
		{
			name: "Domain that matches host has not been verified",
			host: "cz.hcpapps.net",
			dvs: []v1.DomainVerification{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cz.hcpapps.net",
					},
					Spec: v1.DomainVerificationSpec{
						Domain: "cz.hcpapps.net",
					},
					Status: v1.DomainVerificationStatus{
						Verified: false,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cz.false.net",
					},
					Spec: v1.DomainVerificationSpec{
						Domain: "cz.false.net",
					},
					Status: v1.DomainVerificationStatus{
						Verified: true,
					},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDomainVerified(tt.host, tt.dvs)
			if !got == tt.expect {
				t.Errorf("expected '%v' got '%v'", tt.expect, got)
			}
		})
	}
}
