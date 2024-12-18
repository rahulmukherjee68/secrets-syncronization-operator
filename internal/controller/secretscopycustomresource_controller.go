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
	"os"
	"reflect"

	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/rahulmukherjee68/secrets-syncronization-operator/api/v1"
)

// sourceNamespace from where we check if secrets changed or not
var inputNamespace string

// source label key
var inputLabelKey string

// source label value 
var inputLabelValue string

var newSecretFromDiffNameSpace string
var newSecretNameSecretKey string


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
	if err := r.createNewSecretIfAvailable(ctx, newSecretFromDiffNameSpace,newSecretNameSecretKey,inputNamespace); err != nil {
			return ctrl.Result{}, err
	}


	destinationSecretsInstance := &appsv1.SecretsCopyCustomResource{}

	// fetch user passed secrets list
	err := r.Get(ctx, req.NamespacedName, destinationSecretsInstance)

	if err != nil {
		if errors.IsNotFound(err) {
			currlogctx.Info("destinationSecretsInstance not found", "SecretsCopyCustomResource", req.Name, "Namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		currlogctx.Error(err, "Failed to get destinationSecretsInstance")
	}

	currlogctx.Info("Successfully fetched SecretsCopyCustomResource from CR")

	// Iterating over the list of secrets specfied in specs
	for _, secretName := range destinationSecretsInstance.Spec.DestinationSecrets {
		if err := r.createOrUpdateOrDeleteSecrets(ctx, destinationSecretsInstance, secretName, inputNamespace); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}


// creating new secret if avaiable in source with new created namespace
func (r *SecretsCopyCustomResourceReconciler) createNewSecretIfAvailable(ctx context.Context, newNameSpace string,newSecretKey string, sourceNamespace string) error{
	currlogctx := log.FromContext(ctx)
	currlogctx.Info("Into the method createNewSecretIfAvailable", "NewNameSpace", newNameSpace, "NewSecretName", newSecretKey)
	// Get the source secret

	sourceSecretObj := &corev1.Secret{}
	sourceSecretKey := client.ObjectKey{
		Namespace: sourceNamespace,
		Name:      newSecretKey,
	}
	isPresentInSource := true
	// updating sourceSecret instance with k8 client
	if err := r.Get(ctx, sourceSecretKey, sourceSecretObj); err != nil {
		if errors.IsNotFound(err) {
			currlogctx.Info("Source secret not found with Source Namespace ", "SourceNameSpace", sourceNamespace, "SecretName", newSecretKey)
			isPresentInSource = false
			// return nil
		}
	}
	if !isPresentInSource{
		currlogctx.Info("Secrets not found in source ", "SourceNameSpace", sourceNamespace, "SecretName", newSecretKey)
		return nil
	}

	destinationSecretObj := &corev1.Secret{}
	destinationSecretKey := client.ObjectKey{
		Namespace: newNameSpace,
		Name:      newSecretKey,
	}

	if err := r.Get(ctx, destinationSecretKey, destinationSecretObj); err != nil {
		if errors.IsNotFound(err) {
			// create the destination secret if not found in destination host and not deleted
			if !isPresentInSource{
				return r.createSecretWithDiffNameSpace(ctx, newNameSpace, sourceSecretObj)
			}else{
				return nil
			}
		}
		currlogctx.Error(err, "Failed to get destination secret", "Namespace", newNameSpace, "Secret", newSecretKey)
		return err
	}





	return nil
}

// updating source secret to the dentination namespace secret either creating new or updating
func (r *SecretsCopyCustomResourceReconciler) createOrUpdateOrDeleteSecrets(ctx context.Context, secretsCopyCustomResourceInstance *appsv1.SecretsCopyCustomResource, secretName, sourceNamespace string) error {
	currlogctx := log.FromContext(ctx)
	currlogctx.Info("Into the method createOrUpdateSecrets", "SourceNameSpace", sourceNamespace, "SecretName", secretName)

	// Get the source secret
	sourceSecretObj := &corev1.Secret{}
	sourceSecretKey := client.ObjectKey{
		Namespace: sourceNamespace,
		Name:      secretName,
	}
	isDeletedFromSource := false
	// updating sourceSecret instance with k8 client
	if err := r.Get(ctx, sourceSecretKey, sourceSecretObj); err != nil {
		if errors.IsNotFound(err) {
			currlogctx.Info("Source secret not found with Source Namespace ", "SourceNameSpace", sourceNamespace, "SecretName", secretName)
			isDeletedFromSource = true
			// return nil
		} else {
			currlogctx.Error(err, "Failed to get source secret with Source Namespace ", "SourceNameSpace", sourceNamespace, "SecretName", secretName)
			return err
		}
	}

	// Create or update the destination secrets
	destinationSecretObj := &corev1.Secret{}
	destinationSecretKey := client.ObjectKey{
		Namespace: secretsCopyCustomResourceInstance.Namespace,
		Name:      secretName,
	}

	
	if err := r.Get(ctx, destinationSecretKey, destinationSecretObj); err != nil {
		if errors.IsNotFound(err) {
			// create the destination secret if not found in destination host and not deleted
			if !isDeletedFromSource{
				return r.createSecret(ctx, secretsCopyCustomResourceInstance, sourceSecretObj)
			}else{
				return nil
			}
		}
		currlogctx.Error(err, "Failed to get destination secret", "Namespace", secretsCopyCustomResourceInstance.Namespace, "Secret", secretName)
		return err
	}

	if isDeletedFromSource {
		if err := r.Delete(ctx, destinationSecretObj); err != nil {
			currlogctx.Error(err, "Failed to delete unreferenced secret from source", "Namespace", secretsCopyCustomResourceInstance.Namespace, "Name", secretName)
			return err
		}
		return nil
	}
	

	// if values of secrets are not equal then update destination secret
	if !reflect.DeepEqual(sourceSecretObj.Data, destinationSecretObj.Data) {
		// Update the destination secret with source value is different from destination
		return r.updateSecret(ctx, secretsCopyCustomResourceInstance, destinationSecretObj, sourceSecretObj)
	}
	// Destination secret is already up to date
	currlogctx.Info("Source and Destination secrets are Synced Sucessfully")
	return nil
}

// method to update secret from source Namespace to destination Namespace
func (r *SecretsCopyCustomResourceReconciler) updateSecret(ctx context.Context, secretsCopyCustomResourceInstance *appsv1.SecretsCopyCustomResource, destinationSecret, sourceSecret *corev1.Secret) error {
	currlogctx := log.FromContext(ctx)
	dstNamespace := secretsCopyCustomResourceInstance.Namespace
	currlogctx.Info("updating secret in destination namespace ", "DestinationNameSpace", dstNamespace, "SourceSecretName", sourceSecret.Name)

	destinationSecret.Data = sourceSecret.Data
	// Set owner reference to SecretsCopyCustomResource object
	if err := controllerutil.SetControllerReference(secretsCopyCustomResourceInstance, destinationSecret, r.Scheme); err != nil {
		currlogctx.Error(err, "Failed to set owner reference for destination secret file")
		return err
	}
	if err := r.Update(ctx, destinationSecret); err != nil {
		currlogctx.Error(err, "Failed to update Secret with destination namespace", "DestinationNameSpace", dstNamespace, "SourceSecretName", sourceSecret.Name)
		return err
	}
	return nil
}


// method to create a copy of a secret from source Namespace to destination Namespace
func (r *SecretsCopyCustomResourceReconciler) createSecretWithDiffNameSpace(ctx context.Context, newNameSpace string, sourceSecret *corev1.Secret) error {
	currlogctx := log.FromContext(ctx)
	dstNamespace := newNameSpace
	currlogctx.Info("creating secret in destination namespace ", "DestinationNameSpace", dstNamespace, "SourceSecretName", sourceSecret.Name)

	destinationSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecret.Name,
			Namespace: newNameSpace,
		},
		Data: sourceSecret.Data, // Copy data Resource from source to destination secret file
	}
	// Set owner reference to SecretsCopyCustomResource object
	// if err := controllerutil.SetControllerReference(secretsCopyCustomResourceInstance, destinationSecret, r.Scheme); err != nil {
	// 	currlogctx.Error(err, "Failed to set owner reference for destination secret file")
	// 	return err
	// }
	if err := r.Create(ctx, destinationSecret); err != nil {
		currlogctx.Error(err, "Failed to create Secret with destination namespace", "DestinationNameSpace", newNameSpace, "SourceSecretName", sourceSecret.Name)
		return err
	}
	return nil
}

// method to create a copy of a secret from source Namespace to destination Namespace
func (r *SecretsCopyCustomResourceReconciler) createSecret(ctx context.Context, secretsCopyCustomResourceInstance *appsv1.SecretsCopyCustomResource, sourceSecret *corev1.Secret) error {
	currlogctx := log.FromContext(ctx)
	dstNamespace := secretsCopyCustomResourceInstance.Namespace
	currlogctx.Info("creating secret in destination namespace ", "DestinationNameSpace", dstNamespace, "SourceSecretName", sourceSecret.Name)

	destinationSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecret.Name,
			Namespace: secretsCopyCustomResourceInstance.Namespace,
		},
		Data: sourceSecret.Data, // Copy data Resource from source to destination secret file
	}
	// Set owner reference to SecretsCopyCustomResource object
	if err := controllerutil.SetControllerReference(secretsCopyCustomResourceInstance, destinationSecret, r.Scheme); err != nil {
		currlogctx.Error(err, "Failed to set owner reference for destination secret file")
		return err
	}
	if err := r.Create(ctx, destinationSecret); err != nil {
		currlogctx.Error(err, "Failed to create Secret with destination namespace", "DestinationNameSpace", secretsCopyCustomResourceInstance.Namespace, "SourceSecretName", sourceSecret.Name)
		return err
	}
	return nil
}

func (r *SecretsCopyCustomResourceReconciler) destinationPredicate(ctx context.Context) predicate.Predicate {
	// filter our rencoiles for secrets which are not changed
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			currlogctx := log.FromContext(ctx)

			// Fetch objects
			newSecret := &corev1.Secret{}
			sourceSecret := &corev1.Secret{}

			// new secret
			err := r.Get(ctx, client.ObjectKey{
				Namespace: e.ObjectNew.GetNamespace(),
				Name:      e.ObjectNew.GetName(),
			}, newSecret)
			if err != nil {
				currlogctx.Error(err, "Failed to get new secret", "Namespace", e.ObjectNew.GetNamespace(), "Name", e.ObjectNew.GetName())
				return false
			}

			//old secret
			err = r.Get(ctx, client.ObjectKey{
				Namespace: inputNamespace,
				Name:      e.ObjectNew.GetName(),
			}, sourceSecret)
			if err != nil {
				currlogctx.Error(err, "Failed to get source secret", "Namespace", inputNamespace, "Name", e.ObjectNew.GetName())
				return false
			}

			// ignoring reoncile if source secret == destination secret
			if reflect.DeepEqual(sourceSecret.Data, newSecret.Data) {
				currlogctx.Info("secrets Equal not need to change...................")
				return false
			} else {
				currlogctx.Info("Secrets changed need to update destination.....................")
				return true
			}
		},
	}

}

func (r *SecretsCopyCustomResourceReconciler) sourcePredicate(ctx context.Context) predicate.Predicate {
	// filter our rencoiles for secrets which are not changed
	return predicate.Funcs{
		// return true if new secret is created in source namespace
		CreateFunc: func(e event.CreateEvent) bool {

			isSourceNameSpace := e.Object.GetNamespace() == inputNamespace
			if isSourceNameSpace {
				log.FromContext(ctx).Info("Create Event came from source secret")
			}
			return isSourceNameSpace
		},
		// return true if new secret is updated in source namespace
		UpdateFunc: func(e event.UpdateEvent) bool {
			isSourceNameSpace := e.ObjectNew.GetNamespace() == inputNamespace
			if isSourceNameSpace {
				log.FromContext(ctx).Info("Update Event came from source secret")
			}
			return isSourceNameSpace

		},
		// return true if new secret is deleted in source namespace
		DeleteFunc: func(e event.DeleteEvent) bool {
			isSourceNameSpace := e.Object.GetNamespace() == inputNamespace
			if isSourceNameSpace {
				log.FromContext(ctx).Info("Delete Event came from source secret")
			}
			return isSourceNameSpace
		},
	}
}

func (r *SecretsCopyCustomResourceReconciler) handlerFunction(ctx context.Context, o client.Object) []reconcile.Request {

	var destinationSecretsField = ".spec.destinationSecrets"
	currlogctx := log.FromContext(ctx)
	var requests []reconcile.Request

	// check if its a secret type event or not
	secret, err := o.(*corev1.Secret)
	if !err {
		return nil
	}
	
	// Label doesn't match, skip this event request
	if secretValue, exists := secret.Labels[inputLabelKey]; !exists || secretValue != inputLabelValue {
		return nil
	}
	

	// Prepare a list of SecretsCopyCustomResource objects referencing the updated secret
	secretsCopyCustomResourceList := &appsv1.SecretsCopyCustomResourceList{}
	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(destinationSecretsField, secret.GetName()),
	}
	if err := r.List(context.Background(), secretsCopyCustomResourceList, listOpts); err != nil {
		currlogctx.Error(err, "Failed to get secretsCopyCustomResourceList from secret", "Secret", secret.GetName())
		return nil
	}

	// Extract reconcile requests from the found SecretsCopyCustomResource objects
	for _, ss := range secretsCopyCustomResourceList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      ss.GetName(),
				Namespace: ss.GetNamespace(),
			},
		})
	}
	if len(requests) > 0 {
		currlogctx.Info("Got SecretsCopyCustomResource object for the secret ", "Secret", secret.GetName(), "CurrentRequest", requests)
	}

	return requests
}



func (r *SecretsCopyCustomResourceReconciler) namespaceHandlerFunction(ctx context.Context, o client.Object) []reconcile.Request {

	// var destinationSecretsField = ".spec.destinationSecrets"
	inputLabelKey:= "secretname"
	var namespaceValue string
	var exists bool
	currlogctx := log.FromContext(ctx)
	var requests []reconcile.Request

	// check if its a secret type event or not
	namespace, err := o.(*corev1.Namespace)
	if !err {
		return nil
	}
	
	// Label doesn't match, skip this event request
	if namespaceValue, exists = namespace.Labels[inputLabelKey]; !exists {
		return nil
	}
	newSecretFromDiffNameSpace = namespaceValue
	newSecretNameSecretKey = namespace.GetName()

	currlogctx.Info("Added new secret with different namespace", "NameSpace", namespace.GetNamespace(), "SecretKey", newSecretFromDiffNameSpace)
	

	// // Prepare a list of SecretsCopyCustomResource objects referencing the updated secret
	// secretsCopyCustomResourceList := &appsv1.SecretsCopyCustomResourceList{}
	// listOpts := &client.ListOptions{
	// 	FieldSelector: fields.OneTermEqualSelector(destinationSecretsField, secret.GetName()),
	// }
	// if err := r.List(context.Background(), secretsCopyCustomResourceList, listOpts); err != nil {
	// 	currlogctx.Error(err, "Failed to get secretsCopyCustomResourceList from secret", "Secret", secret.GetName())
	// 	return nil
	// }

	// Extract reconcile requests from the found SecretsCopyCustomResource objects
	// for _, ss := range secretsCopyCustomResourceList.Items {
	// 	requests = append(requests, reconcile.Request{
	// 		NamespacedName: client.ObjectKey{
	// 			Name:      ss.GetName(),
	// 			Namespace: ss.GetNamespace(),
	// 		},
	// 	})
	// }
	// if len(requests) > 0 {
	// 	currlogctx.Info("Got SecretsCopyCustomResource object for the secret ", "Secret", secret.GetName(), "CurrentRequest", requests)
	// }


	requests = append(requests, reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      namespace.GetName(),
			Namespace: namespace.GetNamespace(),
		},
	})
	return requests
}




func (r *SecretsCopyCustomResourceReconciler) nameSpacePredicate(ctx context.Context) predicate.Predicate {
	// filter our rencoiles for secrets which are not changed
	return predicate.Funcs{
		// return true if new secret is created in source namespace
		CreateFunc: func(e event.CreateEvent) bool {

			// isSourceNameSpace := e.Object.GetNamespace() == inputNamespace
			// if isSourceNameSpace {
			// 	log.FromContext(ctx).Info("Create Event came from source namespace")
			// }
			return true
		},
	}
}


// SetupWithManager sets up the controller with the Manager.
func (r *SecretsCopyCustomResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	inputNamespace = os.Getenv("INPUT_NAMESPACE")
	if inputNamespace == "" {
		inputNamespace = "default"
	}

	inputLabelKey = os.Getenv("INPUT_LABEL_KEY")
	if inputLabelKey == "" {
		inputLabelKey = "default"
	}

	inputLabelValue = os.Getenv("INPUT_LABEL_VALUE")
	if inputLabelValue == "" {
		inputLabelValue = "default"
	}


	var destinationSecretsField = ".spec.destinationSecrets"
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.SecretsCopyCustomResource{}, destinationSecretsField, func(rawObj client.Object) []string {
		secretsCopyCustomResource := rawObj.(*appsv1.SecretsCopyCustomResource)
		return secretsCopyCustomResource.Spec.DestinationSecrets
	}); err != nil {
		return err
	}

	ctx := context.Background()
	return ctrl.NewControllerManagedBy(mgr).
		// watchs SecretsCopyCustomResource
		For(&appsv1.SecretsCopyCustomResource{}).
		// watcher for DestinationSecrets return true when update is requried
		// Owns(&corev1.Secret{}, builder.WithPredicates(r.destinationPredicate(ctx))).

		// //wathcing secrets in source for every events
		// Watches(
		// 	&corev1.Secret{},
		// 	handler.EnqueueRequestsFromMapFunc(r.handlerFunction),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, r.sourcePredicate(ctx)),
		// ).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.namespaceHandlerFunction),
			builder.WithPredicates(r.nameSpacePredicate(ctx)),
		).
		Complete(r)
}
