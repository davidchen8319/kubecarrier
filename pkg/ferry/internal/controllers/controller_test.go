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

package controllers

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	corev1alpha1 "k8c.io/kubecarrier/pkg/apis/core/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := scheme.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
