package drupalenvironment

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"time"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const drenvCleanupFinalizer = "drupalenvironments.fnresources.acquia.io"

var log = logf.Log.WithName("controller_drupalenvironment")

// Add creates a new DrupalEnvironment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDrupalEnvironment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("drupalenvironment-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 30})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DrupalEnvironment
	err = c.Watch(&source.Kind{Type: &fnv1alpha1.DrupalEnvironment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner DrupalEnvironment
	typesToWatch := []runtime.Object{
		&v1.ConfigMap{},
		// &v1.PersistentVolumeClaim{}, // Probably doesn't need to be watched, as its spec can't be changed
		&v1.Secret{},
		&v1.Service{},
		&appsv1.Deployment{},
		&autoscalingv1.HorizontalPodAutoscaler{},
		&rolloutsv1alpha1.Rollout{},
	}
	for _, t := range typesToWatch {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &fnv1alpha1.DrupalEnvironment{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDrupalEnvironment{}

// ReconcileDrupalEnvironment reconciles a DrupalEnvironment object
type ReconcileDrupalEnvironment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DrupalEnvironment object and makes changes based on the state read
// and what is in the DrupalEnvironment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDrupalEnvironment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)

	// Fetch the DrupalEnvironment instance
	env := &fnv1alpha1.DrupalEnvironment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, env)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			logger.Error(err, "Failed to get DrupalEnvironment")
			return reconcile.Result{}, err
		}
		// Request object not found. Most likely it's been deleted, so do nothing
		return reconcile.Result{}, nil
	}

	if env.Id() == "" {
		env.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), env); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Create a requestHandler instance that will service this reconcile request
	rh := &requestHandler{
		reconciler: r,
		namespace:  request.Namespace,
		env:        env,
		app:        &fnv1alpha1.DrupalApplication{},
		logger:     logger,
	}

	// Check if this resource is being deleted
	isMarkedForDeletion := rh.env.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		// Clean up non-owned Resources
		if os.Getenv("USE_DYNAMIC_PROVISIONING") == "" {
			if requeue, err := rh.finalizePV(); requeue || err != nil {
				return reconcile.Result{Requeue: requeue}, err
			}
		}

		// Remove our finalizer
		if common.RemoveFinalizer(drenvCleanupFinalizer, rh.env) {
			rh.logger.Info("Removing finalizer")
			if err := r.client.Update(context.TODO(), rh.env); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}

	// Ensure our DrupalEnvironment has a finalizer, for cleaning up the PV
	if common.AddFinalizer(drenvCleanupFinalizer, rh.env) {
		rh.logger.Info("Adding finalizer")
		if err := r.client.Update(context.TODO(), rh.env); err != nil {
			rh.logger.Error(err, "Failed to update controller reference")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Label the DrupalEnvironment with the SHA1 hash of its "gitRef" field
	sha := sha1.New()
	sha.Write([]byte(rh.env.Spec.GitRef))
	hashedGitRef := fmt.Sprintf("%x", sha.Sum(nil))

	if rh.env.Labels[fnv1alpha1.GitRefLabel] != hashedGitRef {
		// Update the labels
		rh.env.Labels[fnv1alpha1.GitRefLabel] = hashedGitRef
		err = r.client.Update(context.TODO(), rh.env)
		if err != nil {
			rh.logger.Error(err, "Failed to set git ref label", "Name", rh.env.Name)
			return reconcile.Result{}, err
		}
		rh.logger.Info("Git ref label updated")
		return reconcile.Result{Requeue: true}, nil
	}

	// Fetch the parent DrupalApplication instance
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: env.Spec.Application}, rh.app)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Parent Application doesn't exist", "Application Name", rh.app.Name)
			// Delay the requeue rather than returning an error, to avoid exponential error backoff
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		} else {
			logger.Error(err, "Failed to get Application", "Application Name", rh.app.Name)
			return reconcile.Result{}, err
		}
	}

	// Reconcile owner reference and sync labels from owner
	update, err := common.LinkToOwner(rh.app, rh.env, r.scheme)
	if err != nil {
		return reconcile.Result{}, err
	}
	if update {
		if err := r.client.Update(context.TODO(), rh.env); err != nil {
			rh.logger.Error(err, "Failed to update controller reference")
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Check if domain map secret/configmap exist, otherwise create them
	domainsCM := &v1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "domain-map", Namespace: rh.namespace}, domainsCM)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating domain-map configmap")
		domainsCM = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "domain-map",
				Namespace: rh.namespace,
				Labels:    rh.env.ChildLabels(),
			},
			Data: map[string]string{},
		}
		rh.associateResourceWithController(domainsCM)
		if err := r.client.Create(context.TODO(), domainsCM); err != nil {
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	domainSecret := &v1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "domain-map", Namespace: rh.namespace}, domainSecret)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating domain-map secret")
		domainSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "domain-map",
				Namespace: rh.namespace,
				Labels:    rh.env.ChildLabels(),
			},
			Data: map[string][]byte{},
		}
		rh.associateResourceWithController(domainSecret)
		if err := r.client.Create(context.TODO(), domainSecret); err != nil {
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Check if ConfigMaps exist, otherwise create them
	var requeue bool
	requeue, err = rh.reconcileConfigMap("php-config", map[string]string{
		"drupalcontroller.ini": fmt.Sprintf(`
extension=apcu.so
extension=bcmath.so
extension=bz2.so
extension=calendar.so
extension=dba.so
extension=exif.so
extension=gd.so
extension=gettext.so
extension=gmp.so
extension=gnupg.so
extension=igbinary.so
extension=imagick.so
extension=imap.so
extension=krb5.so
extension=ldap.so
extension=memcached.so
extension=mysqli.so
extension=oauth.so
zend_extension=/usr/local/lib/php/extensions/no-debug-non-zts-20180731/opcache.so
extension=pcntl.so
extension=pdo_dblib.so
extension=pdo_mysql.so
extension=pdo_pgsql.so
extension=pgsql.so
extension=pspell.so
extension=shmop.so
extension=soap.so
extension=sockets.so
extension=sodium.so
extension=sysvmsg.so
extension=sysvsem.so
extension=sysvshm.so
extension=tidy.so
extension=wddx.so
extension=xmlrpc.so
extension=xsl.so
extension=yaml.so
extension=zip.so

extension=raphf.so
extension=propro.so
extension=http.so

memory_limit = %vM
apc.shm_size = %vM
opcache.memory_consumption = %v`,
			rh.env.Spec.Phpfpm.ProcMemoryLimitMiB,
			rh.env.Spec.Phpfpm.ApcMemoryLimitMiB,
			rh.env.Spec.Phpfpm.OpcacheMemoryLimitMiB),
	})
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileConfigMap("phpfpm-config", map[string]string{
		"drupalcontroller.conf": fmt.Sprintf(`
[global]
error_log = /proc/self/fd/2
daemonize = no
emergency_restart_threshold = 10
emergency_restart_interval = 1m
; Wait 10 seconds for a proc to drain before terminating
process_control_timeout = 10s

[www]
; if we send this to /proc/self/fd/1, it never appears
access.log = /proc/self/fd/2
listen = /var/www/php-fpm.sock
pm = static
pm.max_children = %v
pm.max_requests = 500
clear_env = no

; Ensure worker stdout and stderr are sent to the main error log.
catch_workers_output = yes`, rh.env.Spec.Phpfpm.Procs),
	})
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Check if the PV and PVC already exist, if not create them
	isLocal := os.Getenv("USE_DYNAMIC_PROVISIONING")
	if isLocal == "" {
		requeue, err = rh.reconcilePV()
		if err != nil || requeue {
			return reconcile.Result{Requeue: requeue}, err
		}
	}

	requeue, err = rh.reconcilePVC()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileDrupalRollout()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileProxySQLPVC()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileProxySQL()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Check if Services already exists, if not create them
	requeue, err = rh.reconcileDrupalService()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileProxysqlService()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileProxysqlServerConfig()
	if err != nil || requeue {
		if requeue {
			return reconcile.Result{RequeueAfter: time.Second * 1}, nil
		}
		return reconcile.Result{}, err
	}

	requeue, err = rh.reconcileHPA()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	return reconcile.Result{Requeue: requeue}, nil
}

type requestHandler struct {
	reconciler *ReconcileDrupalEnvironment

	env       *fnv1alpha1.DrupalEnvironment
	app       *fnv1alpha1.DrupalApplication
	namespace string
	logger    logr.Logger
}

func (rh *requestHandler) associateResourceWithController(o metav1.Object) {
	err := controllerutil.SetControllerReference(rh.env, o, rh.reconciler.scheme)
	if err != nil {
		rh.logger.Error(err, "Failed to set controller as owner", "Resource", o)
	}
}

func (rh *requestHandler) reconcileConfigMap(name string, data map[string]string) (requeue bool, err error) {
	r := rh.reconciler

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Data: data,
	}

	found := &v1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: rh.namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Set DrupalEnvironment instance as the owner and controller
		rh.associateResourceWithController(cm)

		rh.logger.Info("Creating ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
		err = r.client.Create(context.TODO(), cm)
		if err != nil {
			rh.logger.Error(err, "Failed to create ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile ConfigMap", "Namespace", rh.namespace, "Name", name)
		return false, err
	}

	// Verify that ConfigMap matches expected Spec
	if !cmp.Equal(found.Data, cm.Data) {
		rh.logger.Info("Updating ConfigMap", "Namespace", found.Namespace, "Name", found.Name)

		found.Data = cm.Data

		err = r.client.Update(context.TODO(), found)
	}
	if err != nil {
		rh.logger.Error(err, "Failed to update ConfigMap", "Namespace", found.Namespace, "Name", found.Name)
		return false, err
	}

	return false, nil
}

func (rh *requestHandler) reconcileSecret(name string, stringData map[string]string) (reconcile.Result, error) {
	r := rh.reconciler

	sec := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		StringData: stringData,
	}

	found := &v1.Secret{}
	err := r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: rh.namespace},
		found)

	if err != nil && errors.IsNotFound(err) {
		// Set DrupalEnvironment instance as the owner and controller
		rh.associateResourceWithController(sec)

		rh.logger.Info("Creating Secret", "Namespace", sec.Namespace, "Name", sec.Name)
		err = r.client.Create(context.TODO(), sec)
		if err != nil {
			rh.logger.Error(err, "Failed to create Secret", "Namespace", sec.Namespace, "Name", sec.Name)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile Secret", "Namespace", rh.namespace, "Name", name)
		return reconcile.Result{}, err
	}

	// Verify that Secret matches expected Spec
	if !cmp.Equal(found.Data, sec.Data) {
		rh.logger.Info("Updating Secret", "Namespace", found.Namespace, "Name", found.Name)

		found.Data = sec.Data

		err = r.client.Update(context.TODO(), found)
	}
	if err != nil {
		rh.logger.Error(err, "Failed to update Secret", "Namespace", found.Namespace, "Name", found.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
