package drupalapplication

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
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

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

var log = logf.Log.WithName("controller_drupalapplication")

// Add creates a new DrupalApplication Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDrupalApplication{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("drupalapplication-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DrupalApplication
	err = c.Watch(&source.Kind{Type: &fnv1alpha1.DrupalApplication{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources owned by a DrupalApplication
	typesToWatch := []runtime.Object{
		&fnv1alpha1.DrupalEnvironment{},
	}
	for _, t := range typesToWatch {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &fnv1alpha1.DrupalApplication{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileDrupalApplication implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDrupalApplication{}

// ReconcileDrupalApplication reconciles a DrupalApplication object
type ReconcileDrupalApplication struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

type requestHandler struct {
	reconciler *ReconcileDrupalApplication

	app    *fnv1alpha1.DrupalApplication
	logger logr.Logger
}

// Reconcile reads that state of the cluster for a DrupalApplication object and makes changes based on the state read
// and what is in the DrupalApplication.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDrupalApplication) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	rh := &requestHandler{
		reconciler: r,
		app:        &fnv1alpha1.DrupalApplication{},
		logger:     log.WithValues("Request.Name", request.Name),
	}

	rh.logger.Info("Reconciling DrupalApplication", "Request", request)

	// Fetch the DrupalApplication instance
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: request.NamespacedName.Name}, rh.app)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if rh.app.Id() == "" {
		rh.app.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), rh.app); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Label the DrupalApplication with the SHA1 hash of its "gitRepo" field
	sha := sha1.New()
	sha.Write([]byte(rh.app.Spec.GitRepo))
	hashedGitRepo := fmt.Sprintf("%x", sha.Sum(nil))

	if rh.app.Labels[fnv1alpha1.GitRepoLabel] != hashedGitRepo {
		// Update the labels
		rh.app.Labels[fnv1alpha1.GitRepoLabel] = hashedGitRepo
		err = r.client.Update(context.TODO(), rh.app)
		if err != nil {
			rh.logger.Error(err, "Failed to set git repo label", "Application ID", rh.app.Id())
			return reconcile.Result{}, err
		}
		rh.logger.Info("Git repo label updated")
		return reconcile.Result{Requeue: true}, nil
	}

	// Find all DrupalEnvironments with this Application ID
	drenvList := &fnv1alpha1.DrupalEnvironmentList{}
	labels := map[string]string{fnv1alpha1.ApplicationIdLabel: string(rh.app.Id())}

	err = r.client.List(context.TODO(), client.MatchingLabels(labels), drenvList)
	if err != nil {
		rh.logger.Error(err, "Failed to List DrupalEnvironments by Application ID", "Application ID", rh.app.Id())
		return reconcile.Result{}, err
	}

	// Sort matched environments by Name
	sort.Slice(drenvList.Items, func(i, j int) bool {
		return drenvList.Items[i].Name < drenvList.Items[j].Name
	})

	// Update Status subresource's Environments list
	count := int32(len(drenvList.Items))
	nextStatus := fnv1alpha1.DrupalApplicationStatus{
		NumEnvironments: count,
		Environments:    make([]fnv1alpha1.DrupalEnvironmentRef, count),
	}

	for i, drenv := range drenvList.Items {
		nextStatus.Environments[i].Name = drenv.Name
		nextStatus.Environments[i].Namespace = drenv.Namespace
		nextStatus.Environments[i].UID = drenv.UID
		nextStatus.Environments[i].EnvironmentID = drenv.Labels[fnv1alpha1.EnvironmentIdLabel]
	}

	if !cmp.Equal(nextStatus, rh.app.Status) {
		rh.app.Status = nextStatus
		err = r.client.Status().Update(context.TODO(), rh.app)
		if err != nil {
			rh.logger.Error(err, "Failed to Update DrupalApplication")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (rh *requestHandler) associateResourceWithController(o metav1.Object) {
	err := controllerutil.SetControllerReference(rh.app, o, rh.reconciler.scheme)
	if err != nil {
		rh.logger.Error(err, "Failed to set controller reference", "Resource", o)
	}
}
