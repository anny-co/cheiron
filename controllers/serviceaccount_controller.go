/*
Copyright 2021.

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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ImagePullSecretManagerReconciler reconciles a ImagePullSecretManager object
type ImagePullSecretManagerServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cheiron.anny.co,resources=serviceaccounts,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=serviceaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=serviceaccounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ImagePullSecretManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ImagePullSecretManagerServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// get the manager object in the specific namespace from API server
	serviceAccount := &corev1.ServiceAccount{}
	if err := r.Get(ctx, req.NamespacedName, serviceAccount); err != nil {
		log.Error(err, "Unable to fetch ImagePullSecretManager")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	annotations := serviceAccount.GetAnnotations()

	isReconcilable, isReconcilablePresent := annotations[reconcilableAnnotation]
	reconcileWith, isReconcilableWithPresent := annotations[reconcileWithAnnotation]

	if !isReconcilablePresent || isReconcilable != "true" {
		log.Info("Resource is marked as non-reconcilable", "serviceAccount", serviceAccount.Name)
		return ctrl.Result{}, nil
	}

	if !isReconcilableWithPresent || reconcileWith == "" {
		log.Info("No secrets attached to the resource, not adding secrets", "serviceAccount", serviceAccount.Name)
	}

	secrets := strings.Split(reconcileWith, ",")

	imagePullSecrets := []corev1.LocalObjectReference{}

	for _, s := range secrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: strings.TrimSpace(s),
		})
	}

	serviceAccount.ImagePullSecrets = imagePullSecrets

	// mark serviceAccount as reconciled s.t. later reconciles don't pick up this serviceAccount again (see filters())
	serviceAccount.Annotations[reconciledAnnotation] = "true"

	if err := r.Update(ctx, serviceAccount); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Updated ServiceAccount with imagePullSecrets", "serviceAccount", serviceAccount.Name)

	// TODO(fix): clarify if this operator's actions would clash with flux operator

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
// NOTE: Watching all service accounts for updates might be a little much for our tiny operator, so we
// need to restrict the listeners using predicates
func (r *ImagePullSecretManagerServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(filters()).
		Complete(r)
}
