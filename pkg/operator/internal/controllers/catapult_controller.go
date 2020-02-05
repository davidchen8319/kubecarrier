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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/kubermatic/kubecarrier/pkg/apis/operator/v1alpha1"
	"github.com/kubermatic/kubecarrier/pkg/internal/reconcile"
	resourcescatapult "github.com/kubermatic/kubecarrier/pkg/internal/resources/catapult"
	"github.com/kubermatic/kubecarrier/pkg/internal/util"
)

const (
	catapultControllerFinalizer = "catapult.kubecarrier.io/controller"
)

// CatapultReconciler reconciles a Catapult object
type CatapultReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.kubecarrier.io,resources=catapults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kubecarrier.io,resources=catapults/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete;escalate;bind
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete;escalate;bind
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile function reconciles the Catapult object which specified by the request. Currently, it does the following:
// 1. Fetch the Catapult object.
// 2. Handle the deletion of the Catapult object (Remove the objects that the Catapult owns, and remove the finalizer).
// 3. Reconcile the objects that owned by Catapult object.
// 4. Update the status of the Catapult object.
func (r *CatapultReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("catapult", req.NamespacedName)

	// 1. Fetch the Catapult object.
	catapult := &operatorv1alpha1.Catapult{}
	if err := r.Get(ctx, req.NamespacedName, catapult); err != nil {
		// If the Catapult object is already gone, we just ignore the NotFound error.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Handle the deletion of the Catapult object (Remove the objects that the Catapult owns, and remove the finalizer).
	if !catapult.DeletionTimestamp.IsZero() {
		if err := r.handleDeletion(ctx, catapult); err != nil {
			return ctrl.Result{}, fmt.Errorf("handle deletion: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if util.AddFinalizer(catapult, catapultControllerFinalizer) {
		if err := r.Update(ctx, catapult); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating Catapult finalizers: %w", err)
		}
	}

	// Lookup ServiceClusterRegistration to get name of secret.
	serviceClusterRegistration := &operatorv1alpha1.ServiceClusterRegistration{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      catapult.Spec.ServiceCluster.Name,
		Namespace: catapult.Namespace,
	}, serviceClusterRegistration); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting ServiceClusterRegistration: %w", err)
	}

	// 3. Reconcile the objects that owned by Catapult object.
	// Build the manifests of the Catapult controller manager.
	objects, err := resourcescatapult.Manifests(
		resourcescatapult.Config{
			Name:      catapult.Name,
			Namespace: catapult.Namespace,

			MasterClusterKind:    catapult.Spec.MasterClusterCRD.Kind,
			MasterClusterVersion: catapult.Spec.MasterClusterCRD.Version,
			MasterClusterGroup:   catapult.Spec.MasterClusterCRD.Group,
			MasterClusterPlural:  catapult.Spec.MasterClusterCRD.Plural,

			ServiceClusterKind:    catapult.Spec.ServiceClusterCRD.Kind,
			ServiceClusterVersion: catapult.Spec.ServiceClusterCRD.Version,
			ServiceClusterGroup:   catapult.Spec.ServiceClusterCRD.Group,
			ServiceClusterPlural:  catapult.Spec.ServiceClusterCRD.Plural,

			ServiceClusterName:   catapult.Spec.ServiceCluster.Name,
			ServiceClusterSecret: serviceClusterRegistration.Spec.KubeconfigSecret.Name,
		})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("creating catapult manifests: %w", err)
	}

	deploymentIsReady, err := r.reconcileOwnedObjects(ctx, log, catapult, objects)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 4. Update the status of the Catapult object.
	if err := r.updateCatapultStatus(ctx, catapult, deploymentIsReady); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CatapultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	owner := &operatorv1alpha1.Catapult{}
	enqueuer := util.EnqueueRequestForOwner(owner, mgr.GetScheme())

	return ctrl.NewControllerManagedBy(mgr).
		For(owner).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(&source.Kind{Type: &rbacv1.ClusterRole{}}, enqueuer).
		Watches(&source.Kind{Type: &rbacv1.ClusterRoleBinding{}}, enqueuer).
		Complete(r)
}

// handleDeletion handles the deletion of the Catapult object. Currently, it does:
// 1. Update the Catapult status to Terminating.
// 2. Delete the objects that the Catapult object owns.
// 3. Remove the finalizer from the Catapult object.
func (r *CatapultReconciler) handleDeletion(ctx context.Context, kubeCarrier *operatorv1alpha1.Catapult) error {

	// 1. Update the Catapult Status to Terminating.
	readyCondition, _ := kubeCarrier.Status.GetCondition(operatorv1alpha1.CatapultReady)
	if readyCondition.Status != operatorv1alpha1.ConditionFalse ||
		readyCondition.Status == operatorv1alpha1.ConditionFalse && readyCondition.Reason != operatorv1alpha1.CatapultTerminatingReason {
		kubeCarrier.Status.ObservedGeneration = kubeCarrier.Generation
		kubeCarrier.Status.SetCondition(operatorv1alpha1.CatapultCondition{
			Type:    operatorv1alpha1.CatapultReady,
			Status:  operatorv1alpha1.ConditionFalse,
			Reason:  operatorv1alpha1.CatapultTerminatingReason,
			Message: "Catapult is being terminated",
		})
		if err := r.Status().Update(ctx, kubeCarrier); err != nil {
			return fmt.Errorf("updating Catapult status: %w", err)
		}
	}

	// 2. Delete Objects.
	ownedByFilter := util.OwnedBy(kubeCarrier, r.Scheme)

	clusterRoleBindingsCleaned, err := cleanupClusterRoleBindings(ctx, r.Client, ownedByFilter)
	if err != nil {
		return fmt.Errorf("cleaning ClusterRoleBinding: %w", err)
	}

	clusterRolesCleaned, err := cleanupClusterRoles(ctx, r.Client, ownedByFilter)
	if err != nil {
		return fmt.Errorf("cleaning ClusterRoles: %w", err)
	}

	customResourceDefinitionsCleaned, err := cleanupCustomResourceDefinitions(ctx, r.Client, ownedByFilter)
	if err != nil {
		return fmt.Errorf("cleaning CustomResourceDefinitions: %w", err)
	}

	// Make sure all the ClusterRoleBindings, ClusterRoles and CustomResourceDefinitions are deleted.
	if !clusterRoleBindingsCleaned || !clusterRolesCleaned || !customResourceDefinitionsCleaned {
		return nil
	}

	// 3. Remove the Finalizer.
	if util.RemoveFinalizer(kubeCarrier, catapultControllerFinalizer) {
		if err := r.Update(ctx, kubeCarrier); err != nil {
			return fmt.Errorf("updating Catapult finalizers: %w", err)
		}
	}
	return nil
}

// reconcileOwnedObjects adds the OwnerReference to the objects that owned by this Catapult object, and reconciles them.
func (r *CatapultReconciler) reconcileOwnedObjects(ctx context.Context, log logr.Logger, kubeCarrier *operatorv1alpha1.Catapult, objects []unstructured.Unstructured) (bool, error) {
	var deploymentIsReady bool
	for _, object := range objects {
		if err := addOwnerReference(kubeCarrier, &object, r.Scheme); err != nil {
			return false, err
		}
		curObj, err := reconcile.Unstructured(ctx, log, r.Client, &object)
		if err != nil {
			return false, fmt.Errorf("reconcile kind: %s, err: %w", object.GroupVersionKind().Kind, err)
		}

		switch obj := curObj.(type) {
		case *appsv1.Deployment:
			deploymentIsReady = util.DeploymentIsAvailable(obj)
		}
	}
	return deploymentIsReady, nil
}

// updateCatapultStatus updates the Status of the Catapult object if needed.
func (r *CatapultReconciler) updateCatapultStatus(ctx context.Context, kubeCarrier *operatorv1alpha1.Catapult, deploymentIsReady bool) error {
	var updateStatus bool
	readyCondition, _ := kubeCarrier.Status.GetCondition(operatorv1alpha1.CatapultReady)
	if !deploymentIsReady && readyCondition.Status != operatorv1alpha1.ConditionFalse {
		updateStatus = true
		kubeCarrier.Status.ObservedGeneration = kubeCarrier.Generation
		kubeCarrier.Status.SetCondition(operatorv1alpha1.CatapultCondition{
			Type:    operatorv1alpha1.CatapultReady,
			Status:  operatorv1alpha1.ConditionFalse,
			Reason:  "DeploymentUnready",
			Message: "the deployment of the Catapult controller manager is not ready",
		})
	}

	if deploymentIsReady && readyCondition.Status != operatorv1alpha1.ConditionTrue {
		updateStatus = true
		kubeCarrier.Status.ObservedGeneration = kubeCarrier.Generation
		kubeCarrier.Status.SetCondition(operatorv1alpha1.CatapultCondition{
			Type:    operatorv1alpha1.CatapultReady,
			Status:  operatorv1alpha1.ConditionTrue,
			Reason:  "DeploymentReady",
			Message: "the deployment of the Catapult controller manager is ready",
		})
	}

	if updateStatus {
		if err := r.Status().Update(ctx, kubeCarrier); err != nil {
			return fmt.Errorf("updating Catapult status: %w", err)
		}
	}
	return nil
}