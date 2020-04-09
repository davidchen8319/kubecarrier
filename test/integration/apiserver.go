/*
Copyright 2019 The KubeCarrier Authors.

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

package integration

import (
	"context"
	"strings"
	"testing"

	certmanagerv1alpha3 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha3"
	v1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kubermatic/kubecarrier/pkg/apis/operator/v1alpha1"
	"github.com/kubermatic/kubecarrier/pkg/testutil"
)

func newAPIServer(f *testutil.Framework) func(t *testing.T) {
	return func(t *testing.T) {
		t.Log("testing how API Server works")
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		managementClient, err := f.ManagementClient(t)
		require.NoError(t, err, "creating management client")
		t.Cleanup(managementClient.CleanUpFunc(ctx))

		testName := strings.Replace(strings.ToLower(t.Name()), "/", "-", -1)

		ns := &corev1.Namespace{}
		ns.Name = testName
		require.NoError(t, managementClient.Create(ctx, ns))

		servingTLSSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-tls",
				Namespace: ns.GetName(),
			},
		}

		issuer := &certmanagerv1alpha3.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: ns.GetName(),
			},
			Spec: certmanagerv1alpha3.IssuerSpec{
				IssuerConfig: certmanagerv1alpha3.IssuerConfig{
					SelfSigned: &certmanagerv1alpha3.SelfSignedIssuer{},
				},
			},
		}
		cert := &certmanagerv1alpha3.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: ns.GetName(),
			},
			Spec: certmanagerv1alpha3.CertificateSpec{
				SecretName: servingTLSSecret.GetName(),
				DNSNames: []string{
					strings.Join([]string{"foo", servingTLSSecret.GetNamespace(), "svc"}, "."),
				},
				IssuerRef: v1.ObjectReference{
					Name: issuer.GetName(),
				},
			},
		}
		require.NoError(t, managementClient.Create(ctx, issuer))
		require.NoError(t, managementClient.Create(ctx, cert))
		require.NoError(t, testutil.WaitUntilReady(ctx, managementClient, cert), "cert not ready")
		require.NoError(t, managementClient.WaitUntil(ctx, servingTLSSecret, func() (done bool, err error) {
			data, ok := servingTLSSecret.Data[corev1.TLSCertKey]
			return ok && len(data) > 0, nil
		}))

		apiServer := &operatorv1alpha1.APIServer{ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: ns.GetName(),
		},
			Spec: operatorv1alpha1.APIServerSpec{
				TLSSecretRef: operatorv1alpha1.ObjectReference{
					Name: servingTLSSecret.GetName(),
				},
			},
		}
		require.NoError(t, managementClient.Create(ctx, apiServer))
		assert.NoError(t, testutil.WaitUntilReady(ctx, managementClient, apiServer))
	}
}
