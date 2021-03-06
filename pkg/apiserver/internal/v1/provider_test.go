/*
Copyright 2020 The KubeCarrier Authors.

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

package v1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	catalogv1alpha1 "k8c.io/kubecarrier/pkg/apis/catalog/v1alpha1"
	v1 "k8c.io/kubecarrier/pkg/apiserver/api/v1"
)

func TestListProvider(t *testing.T) {
	providers := &catalogv1alpha1.ProviderList{
		Items: []catalogv1alpha1.Provider{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-provider-1",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label": "provider1",
					},
				},
				Spec: catalogv1alpha1.ProviderSpec{
					Metadata: catalogv1alpha1.AccountMetadata{
						CommonMetadata: catalogv1alpha1.CommonMetadata{
							Description: "Test Provider",
							DisplayName: "Test Provider",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-provider-2",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"test-label": "provider2",
					},
				},
				Spec: catalogv1alpha1.ProviderSpec{
					Metadata: catalogv1alpha1.AccountMetadata{
						CommonMetadata: catalogv1alpha1.CommonMetadata{
							Description: "Test Provider",
							DisplayName: "Test Provider",
						},
					},
				},
			},
		},
	}
	client := fakeclient.NewFakeClientWithScheme(testScheme, providers)
	providerServer := providerServer{
		client: client,
	}
	ctx := context.Background()
	tests := []struct {
		name           string
		req            *v1.ListRequest
		expectedError  error
		expectedResult *v1.ProviderList
	}{
		{
			name: "valid request",
			req: &v1.ListRequest{
				Account: "test-namespace",
			},
			expectedError: nil,
			expectedResult: &v1.ProviderList{
				Metadata: &v1.ListMeta{
					Continue:        "",
					ResourceVersion: "",
				},
				Items: []*v1.Provider{
					{
						Metadata: &v1.ObjectMeta{
							Name:    "test-provider-1",
							Account: "test-namespace",
							Labels: map[string]string{
								"test-label": "provider1",
							},
						},
						Spec: &v1.ProviderSpec{
							Metadata: &v1.ProviderMetadata{
								Description: "Test Provider",
								DisplayName: "Test Provider",
							},
						},
					},
					{
						Metadata: &v1.ObjectMeta{
							Name:    "test-provider-2",
							Account: "test-namespace",
							Labels: map[string]string{
								"test-label": "provider2",
							},
						},
						Spec: &v1.ProviderSpec{
							Metadata: &v1.ProviderMetadata{
								Description: "Test Provider",
								DisplayName: "Test Provider",
							},
						},
					},
				},
			},
		},
		{
			name: "LabelSelector works",
			req: &v1.ListRequest{
				Account:       "test-namespace",
				LabelSelector: "test-label=provider1",
			},
			expectedError: nil,
			expectedResult: &v1.ProviderList{
				Metadata: &v1.ListMeta{
					Continue:        "",
					ResourceVersion: "",
				},
				Items: []*v1.Provider{
					{
						Metadata: &v1.ObjectMeta{
							Name:    "test-provider-1",
							Account: "test-namespace",
							Labels: map[string]string{
								"test-label": "provider1",
							},
						},
						Spec: &v1.ProviderSpec{
							Metadata: &v1.ProviderMetadata{
								Description: "Test Provider",
								DisplayName: "Test Provider",
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providers, err := providerServer.List(ctx, test.req)
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedResult, providers)
		})
	}
}

func TestGetProvider(t *testing.T) {
	provider := &catalogv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider",
			Namespace: "test-namespace",
		},
		Spec: catalogv1alpha1.ProviderSpec{
			Metadata: catalogv1alpha1.AccountMetadata{
				CommonMetadata: catalogv1alpha1.CommonMetadata{
					Description: "Test Provider",
					DisplayName: "Test Provider",
				},
			},
		},
	}
	client := fakeclient.NewFakeClientWithScheme(testScheme, provider)
	providerServer := providerServer{
		client: client,
	}
	ctx := context.Background()
	tests := []struct {
		name           string
		req            *v1.GetRequest
		expectedError  error
		expectedResult *v1.Provider
	}{
		{
			name: "valid request",
			req: &v1.GetRequest{
				Name:    "test-provider",
				Account: "test-namespace",
			},
			expectedError: nil,
			expectedResult: &v1.Provider{
				Metadata: &v1.ObjectMeta{
					Name:    "test-provider",
					Account: "test-namespace",
				},
				Spec: &v1.ProviderSpec{
					Metadata: &v1.ProviderMetadata{
						Description: "Test Provider",
						DisplayName: "Test Provider",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := providerServer.Get(ctx, test.req)
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedResult, provider)
		})
	}
}
