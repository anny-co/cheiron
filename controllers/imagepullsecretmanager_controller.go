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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cheironv1alpha1 "github.com/anny-co/cheiron/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	types "k8s.io/apimachinery/pkg/types"
)

// ImagePullSecretManagerReconciler reconciles a ImagePullSecretManager object
type ImagePullSecretManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cheiron.anny.co,resources=imagepullsecretmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=pods,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=serviceaccounts,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=imagepullsecretmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cheiron.anny.co,resources=imagepullsecretmanagers/finalizers,verbs=update

var reconcilableAnnotation = "cheiron.anny.co/reconcilable"
var ignoreAnnotation = "cheiron.anny.co/ignore"
var reconcileWithAnnotation = "cheiron.anny.co/reconcile-with"

// filters() filters events to reduce load on Pod updates where the reconciled annotation is set
// by the operator
//
// By default, reconciles all fresh elements, but only reconciles updated ressources if the annotation
// cheiron.anny.co/reconciled is undefined or set to False. Deleted objects are not reconciled at all.
func filters() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			annotations := e.ObjectOld.GetAnnotations()
			val, ok := annotations["cheiron.anny.co/is-reconciled"]
			return !ok || val == "False"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

// updatePodAnnotations adds the common labels for cheiron on pods to a given pod ressource
func updatePodAnnotations(pod corev1.Pod, secrets string) corev1.Pod {
	annotations := pod.GetAnnotations()

	_, reconcilablePresent := annotations[reconcilableAnnotation]
	ignore, ignorePresent := annotations[ignoreAnnotation]
	if ignorePresent && ignore == "true" {
		return pod
	}
	if !reconcilablePresent {
		// pod currently is not marked as reconcilable, add the annotation!
		pod.Annotations[reconcilableAnnotation] = "true"
		pod.Annotations[ignoreAnnotation] = "false"
		pod.Annotations[reconcileWithAnnotation] = secrets
	}
	return pod
}

// updatePodAnnotations adds the common labels for cheiron on pods to a given pod ressource
func updateServiceAccountAnnotations(sa corev1.ServiceAccount, secrets string) corev1.ServiceAccount {
	annotations := sa.GetAnnotations()

	_, reconcilablePresent := annotations[reconcilableAnnotation]
	ignore, ignorePresent := annotations[ignoreAnnotation]
	if ignorePresent && ignore == "true" {
		return sa
	}
	if !reconcilablePresent {
		// pod currently is not marked as reconcilable, add the annotation!
		sa.Annotations[reconcilableAnnotation] = "true"
		sa.Annotations[ignoreAnnotation] = "false"
		sa.Annotations[reconcileWithAnnotation] = secrets
	}
	return sa
}

// Reconciles all pods in the request's namespace s.t. they have the set of required annotations for the pod controller of
// Cheiron already set
func (r *ImagePullSecretManagerReconciler) GetAndUpdatePods(ctx context.Context, req ctrl.Request, secrets string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "Failed to fetch all pods in namespace")
		return ctrl.Result{}, err
	}

	for _, pod := range pods.Items {
		pod = updatePodAnnotations(pod, secrets)
	}

	return ctrl.Result{}, nil
}

// Reconciles all service accounts in the request's namespace s.t. they have the set of required annotations for the pod controller of
// Cheiron already set
func (r *ImagePullSecretManagerReconciler) GetAndUpdateServiceAccounts(ctx context.Context, req ctrl.Request, secrets string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var serviceAccounts corev1.ServiceAccountList
	if err := r.List(ctx, &serviceAccounts, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "Failed to fetch all service accounts in namespace")
		return ctrl.Result{}, err
	}

	for _, pod := range serviceAccounts.Items {
		pod = updateServiceAccountAnnotations(pod, secrets)
	}

	return ctrl.Result{}, nil
}

// CreateOrUpdateSecret fetches an existing secret with the name specified in the CR or creates a new one,
// adds the registry credentials as payload and (re-)submits it to the API server
func (r *ImagePullSecretManagerReconciler) CreateOrUpdateSecret(ctx context.Context, req ctrl.Request, manager *cheironv1alpha1.ImagePullSecretManager, pullSecret *cheironv1alpha1.ImagePullSecretSpec) (*corev1.Secret, error) {
	log := log.FromContext(ctx)
	create := false
	name := types.NamespacedName{Name: pullSecret.Name, Namespace: req.Namespace}
	existingSecret := &corev1.Secret{}

	err := r.Get(ctx, name, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// no secret with that name exists, create a new one!
			create = true
			existingSecret = newDockerSecretObj(name.Name, req.Namespace)
		} else {
			return nil, err
		}
	}

	username := pullSecret.Username
	password := pullSecret.Password
	email := pullSecret.Email
	registry := pullSecret.Registry
	dockerConfigJSONContent, err := handleDockerCfgJSONContent(username, password, email, registry)

	if err != nil {
		log.Error(err, "Failed to create secret from CRD")
		return nil, err
	}

	existingSecret.Data[corev1.DockerConfigJsonKey] = dockerConfigJSONContent

	if err := ctrl.SetControllerReference(manager, existingSecret, r.Scheme); err != nil {
		return nil, err
	}

	if create {
		if err := r.Create(ctx, existingSecret); err != nil {
			return nil, err
		}
	} else {
		if err := r.Update(ctx, existingSecret); err != nil {
			return nil, err
		}
	}
	return existingSecret, nil
}

// secretIsFullySpecified is a validator function for a ImagePullSecretSpec that returns either true if the secret spec is sufficient or
// false if not
func secretIsFullySpecified(secret *cheironv1alpha1.ImagePullSecretSpec) bool {
	if secret.ExistingSecretRef.Name == "" {
		if secret.Name != "" && secret.Email != "" && secret.Password != "" && secret.Username != "" && secret.Registry != "" {
			return true
		}
		return false
	}
	return true
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ImagePullSecretManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ImagePullSecretManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// get the manager object in the specific namespace from API server
	imgr := &cheironv1alpha1.ImagePullSecretManager{}
	if err := r.Get(ctx, req.NamespacedName, imgr); err != nil {
		log.Error(err, "Unable to fetch ImagePullSecretManager")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO(fix): add fallthrough for neither, existingSecretRef, or full specification of creds being present
	secretNames := []string{}
	for _, secret := range imgr.Spec.Secrets {
		if !secretIsFullySpecified(&secret) {
			// skip this secret as it is not fully specified
			log.Error(errors.NewBadRequest("Secret not fully specified"), "ImagePullSecret is not fully specified", "imagePullSecretManager", imgr.Name)
			continue
		}
		if secret.ExistingSecretRef.Name != "" {
			// existing secret ref present as localObjectReference, just add the name to the string for annotation
			secretNames = append(secretNames, secret.ExistingSecretRef.Name)
		} else {
			// create new dockerconfigjson secret from the given name if it does not exist, and update its payload
			secretObj, err := r.CreateOrUpdateSecret(ctx, req, imgr, &secret)
			if err != nil {
				return ctrl.Result{}, err
			}
			secretNames = append(secretNames, secretObj.Name)
		}
	}

	s := strings.Join(secretNames, ",")

	// Depending on the mode, mark all "mode" resources in the namespace as reconcilable with
	// the LocalObjectReference name set as annotation to consume from either PodController or
	// ServiceAccountController

	mode := imgr.Spec.Mode
	if mode == cheironv1alpha1.PodMode {
		return r.GetAndUpdatePods(ctx, req, s)
	} else if mode == cheironv1alpha1.ServiceAccountMode {
		return r.GetAndUpdateServiceAccounts(ctx, req, s)
	} else {
		err := errors.NewBadRequest("Value of mode spec is not supported")
		log.Error(err, "Unsupported mode")
		return ctrl.Result{}, err
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImagePullSecretManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cheironv1alpha1.ImagePullSecretManager{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
