package muminiobucket

import (
	"context"
	"os"
	"strings"

	muminiov1alpha1 "github.com/cbenien/muminio/pkg/apis/muminio/v1alpha1"
	"github.com/minio/minio/pkg/madmin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_muminiobucket")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MuminioBucket Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMuminioBucket{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("muminiobucket-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MuminioBucket
	err = c.Watch(&source.Kind{Type: &muminiov1alpha1.MuminioBucket{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MuminioBucket
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &muminiov1alpha1.MuminioBucket{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMuminioBucket implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMuminioBucket{}

// ReconcileMuminioBucket reconciles a MuminioBucket object
type ReconcileMuminioBucket struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MuminioBucket object and makes changes based on the state read
// and what is in the MuminioBucket.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMuminioBucket) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MuminioBucket")

	// Fetch the MuminioBucket instance
	instance := &muminiov1alpha1.MuminioBucket{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	minioURL := os.Getenv("MINIO_URL")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioSecure := strings.ToLower(os.Getenv("MINIO_SECURE")) == "true"

	accessKey := "newuser"
	secretKey := "supersecret"

	// Define a new Secret object
	secret := newSecretForCR(instance, accessKey, secretKey)

	// Set MuminioBucket instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Secret already exists
	found := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			return reconcile.Result{}, err
		}

	} else if err != nil {
		return reconcile.Result{}, err
	} else {
		// Secret already exists - don't requeue
		reqLogger.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}

	reqLogger.Info("Creating user...")

	minioAdminClient, err := madmin.New(minioURL, minioAccessKey, minioSecretKey, minioSecure)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = minioAdminClient.AddUser(accessKey, secretKey)
	if err != nil {
		reqLogger.Error(err, "Can't create user", "AccessKey", accessKey)
	}

	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newSecretForCR(cr *muminiov1alpha1.MuminioBucket, accessKey string, secretKey string) *corev1.Secret {
	labels := map[string]string{
		"app": cr.Name,
	}

	data := map[string]string{
		"accessKey": accessKey,
		"secretKey": secretKey,
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-secret",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Type:       "Opaque",
		StringData: data,
	}
}
