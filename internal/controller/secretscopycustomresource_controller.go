/*
Copyright 2024.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	errors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/rahulmukherjee68/secrets-syncronization-operator/api/v1"
)

// sourceNamespace from where we check if secrets changed or not
var sourceNamespace string

// SecretsCopyCustomResourceReconciler reconciles a SecretsCopyCustomResource object
type SecretsCopyCustomResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.example.com,resources=secretscopycustomresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.example.com,resources=secretscopycustomresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.example.com,resources=secretscopycustomresources/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretsCopyCustomResource object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *SecretsCopyCustomResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	currlogctx := log.FromContext(ctx)

	currlogctx.Info("Handler came to rencile method to be check if secrets changed or not")

	// get the SecretsCopyCustomResource instance for destinationSecrets names to be replaced in the destination namespace
	destinationSecretsInstance := &appsv1.SecretsCopyCustomResource{}

	// fetch user passed secrets list
	err := r.Get(ctx, req.NamespacedName, destinationSecretsInstance)

	if err != nil{
		if errors.IsNotFound(err) {
			currlogctx.Info("destinationSecretsInstance not found", "SecretsCopyCustomResource", req.Name, "Namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		currlogctx.Error(err, "Failed to get destinationSecretsInstance")
	}

	currlogctx.Info("Successfully fetched SecretsCopyCustomResource from CR")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretsCopyCustomResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.SecretsCopyCustomResource{}).
		Owns(&corev1.Secret{}). //wathcing secrets to be changed
		Complete(r)
}
