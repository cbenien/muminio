package muminiobucket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	gerrors "errors"
	"os"
	"strings"
	"time"

	muminiov1alpha1 "github.com/cbenien/muminio/pkg/apis/muminio/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v6"
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

	return &ReconcileMuminioBucket{
		client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		minioURL:       os.Getenv("MINIO_URL"),
		minioAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		minioSecretKey: os.Getenv("MINIO_SECRET_KEY"),
		minioSecure:    strings.ToLower(os.Getenv("MINIO_SECURE")) == "true",
	}
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
	// Watch for changes to secondary resource Secrets and requeue the owner MuminioBucket
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
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

	minioURL       string
	minioAccessKey string
	minioSecretKey string
	minioSecure    bool
}

func randomString(len int) string {

	b := make([]byte, len)
	rand.Read(b)

	encoded := make([]byte, hex.EncodedLen(len))
	hex.Encode(encoded, b)

	return string(encoded)
}

func (r *ReconcileMuminioBucket) createBucket(reqLogger logr.Logger, bucketName string) (bool, error) {

	minioClient, err := minio.New(r.minioURL, r.minioAccessKey, r.minioSecretKey, r.minioSecure)
	if err != nil {
		return false, err
	}

	bucketExists, err := minioClient.BucketExists(bucketName)
	if err != nil {
		return false, err
	}

	if !bucketExists {
		reqLogger.Info("Creating bucket", "BucketName", bucketName)
		err = minioClient.MakeBucket(bucketName, "us-east-1")
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
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

	r.createBucket(reqLogger, instance.Name)

	accessKey := randomString(16)
	secretKey := randomString(32)

	// Define a new Secret object
	secret := newSecretForCR(instance, accessKey, secretKey)

	// Check if this Secret already exists
	found := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Set MuminioBucket instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	} else {

		// read Secret from Kubernetes API server

		if _, ok := found.Data["accessKey"]; !ok {
			err = gerrors.New("Secret does not contain key 'accessKey'")
			reqLogger.Error(err, err.Error(), "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
			return reconcile.Result{}, err
		}

		if _, ok := found.Data["secretKey"]; !ok {
			err = gerrors.New("Secret does not contain key 'secretKey'")
			reqLogger.Error(err, err.Error(), "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
			return reconcile.Result{}, err
		}

		accessKey = string(found.Data["accessKey"])
		secretKey = string(found.Data["secretKey"])

		reqLogger.Info("Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name, "AccessKey", accessKey)
	}

	reqLogger.Info("Connecting to Minio admin endpoint...", "Minio.URL", r.minioURL)
	minioAdminClient, err := madmin.New(r.minioURL, r.minioAccessKey, r.minioSecretKey, r.minioSecure)
	if err != nil {
		return reconcile.Result{}, err
	}

	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::BUCKETNAME/*"]}]}`
	policy = strings.ReplaceAll(policy, "BUCKETNAME", instance.Name)
	policyName := "policy-" + instance.Name

	// Create or update user
	existingUser, err := minioAdminClient.GetUserInfo(accessKey)
	if err != nil {
		//TODO: parse error - for now: assume user does not exist
		reqLogger.Info("User not found", "Error", err)

		reqLogger.Info("Creating user...", "User.AccessKey", accessKey)
		err = minioAdminClient.AddUser(accessKey, secretKey)
		if err != nil {
			reqLogger.Error(err, "Can't create user", "User.AccessKey", accessKey)
			return reconcile.Result{}, err
		}
	} else {
		reqLogger.Info("Existing user", "User.PolicyName", existingUser.PolicyName)

		userCredentialsOk := true
		minioUserClient, err := minio.New(r.minioURL, accessKey, secretKey, r.minioSecure)
		if err != nil {
			reqLogger.Error(err, "Failed to create Minio client with user credentials")
			userCredentialsOk = false
		} else {
			_, err = minioUserClient.BucketExists(instance.Name)
			if err != nil {
				reqLogger.Error(err, "Failed to get buckets with existing credentials")
				userCredentialsOk = false
			}
		}

		if !userCredentialsOk {

			// List all existing CRD instances in all namespaces and make sure that this is not a hijack attempt
			// Users and buckets are shared on Minio level but we want isolation by namespace at least
			// of course we want to allow migration of workloads to new namespaces, so if they provide the correct
			// secretKey, it's fine

			muminioList := muminiov1alpha1.MuminioBucketList{}
			r.client.List(context.TODO(), &muminioList)

			for _, mmb := range muminioList.Items {
				if mmb.Status.MinioAccessKey == accessKey && (mmb.Namespace != instance.Namespace || mmb.Name != instance.Name) {
					err := errors.NewBadRequest("Can't update user account, it's owned by another instance")
					reqLogger.Error(err, "Unable to update user account, it's owned by another instance", "OwningInstance",
						mmb.Namespace+"/"+mmb.Name)
					return reconcile.Result{RequeueAfter: time.Minute}, nil
				}
			}

			reqLogger.Info("SecretKey has changed, recreating user...", "User.AccessKey", accessKey)
			err = minioAdminClient.RemoveUser(accessKey)
			if err != nil {
				reqLogger.Error(err, "Can't remove user", "User.AccessKey", accessKey)
				return reconcile.Result{}, err
			}

			err = minioAdminClient.AddUser(accessKey, secretKey)
			if err != nil {
				reqLogger.Error(err, "Can't create user", "User.AccessKey", accessKey)
				return reconcile.Result{}, err
			}
		}
	}

	// Delete old user if it has changed
	if accessKey != instance.Status.MinioAccessKey {
		reqLogger.Info("Removing old user", "AccessKey", instance.Status.MinioAccessKey)
		err = minioAdminClient.RemoveUser(instance.Status.MinioAccessKey)
		if err != nil {
			reqLogger.Error(err, "Unable to remove user", "AccessKey", instance.Status.MinioAccessKey)
		}
	}

	existingPolicy, err := minioAdminClient.InfoCannedPolicy(policyName)
	if err != nil {
		reqLogger.Info("Policy doesn't exist", "Policy.Name", policyName)

		reqLogger.Info("Creating policy...", "Policy.Name", policyName, "Policy.Data", policy)
		err = minioAdminClient.AddCannedPolicy(policyName, policy)
		if err != nil {
			reqLogger.Error(err, "Can't create policy", "Policy.Name", policyName, "Policy.Data", policy)
			return reconcile.Result{}, err
		}

	} else {

		reqLogger.Info("Existing policy", "Policy.Name", policyName, "Policy.Data", string(existingPolicy))

		existingPolicyStr := string(existingPolicy)
		if policy != existingPolicyStr {
			reqLogger.Info("Existing policy will be replaced", "ExpectedPolicy", policy, "ActualPolicy", existingPolicyStr)

			reqLogger.Info("Removing policy...", "Policy.Name", policyName)
			err = minioAdminClient.RemoveCannedPolicy(policyName)
			if err != nil {
				reqLogger.Error(err, "Can't remove policy", "Policy.Name", policyName)
				return reconcile.Result{}, err
			}

			reqLogger.Info("Creating policy...", "Policy.Name", policyName, "Policy.Data", policy)
			err = minioAdminClient.AddCannedPolicy(policyName, policy)
			if err != nil {
				reqLogger.Error(err, "Can't create policy", "Policy.Name", policyName, "Policy.Data", policy)
				return reconcile.Result{}, err
			}
		}
	}

	reqLogger.Info("Assigning policy...", "Policy.Name", policyName, "User.Name", accessKey)
	err = minioAdminClient.SetPolicy(policyName, accessKey, false)
	if err != nil {
		reqLogger.Error(err, "Can't assign policy", "Policy.Name", policyName, "User.Name", accessKey)
		return reconcile.Result{}, err
	}

	if r.minioSecure {
		instance.Status.MinioURL = "https://" + r.minioURL
	} else {
		instance.Status.MinioURL = "http://" + r.minioURL
	}

	instance.Status.MinioAccessKey = accessKey

	reqLogger.Info("Updating CRD status")
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Unable to update status")
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
			Name:      cr.Spec.SecretName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Type:       "Opaque",
		StringData: data,
	}
}
