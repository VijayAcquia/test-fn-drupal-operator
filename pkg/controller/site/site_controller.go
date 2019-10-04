package site

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
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

	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

// siteCleanupFinalizer defines the site finalizer.
const siteCleanupFinalizer = "sites.fnresources.acquia.io"

// dbPwdSecretFinalizer defines the database password secret finalizer.
const dbPwdSecretFinalizer = "sites.fnresources.acquia.com/password"

var log = logf.Log.WithName("controller_site")

// Add creates a new Site Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSite{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("site-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 30})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Site
	// err = c.Watch(&source.Kind{Type: &fn.Site{}}, &handler.EnqueueRequestsFromMapFunc{})
	if err := c.Watch(&source.Kind{Type: &fn.Site{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for secondary resources created by and owned exclusively by a Site
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &fn.Site{},
	}); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &extv1b1.Ingress{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &fn.Site{},
	}); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &batchv1b1.CronJob{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &fn.Site{},
	}); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSite implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSite{}

// ReconcileSite reconciles a Site object
type ReconcileSite struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// requestHandler gets initialized per request to have thread-safe code.
type requestHandler struct {
	reconciler *ReconcileSite

	app    *fn.DrupalApplication
	env    *fn.DrupalEnvironment
	site   *fn.Site
	logger logr.Logger
}

// Reconcile reads that state of the cluster for a Site object and makes changes based on the state read
// and what is in the Site.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSite) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)

	// Fetch the Site instance
	site := &fn.Site{}
	err := r.client.Get(context.TODO(), request.NamespacedName, site)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			reqLogger.Error(err, "Failed to Get Site")
			return reconcile.Result{}, err
		}
		// Request object not found. Most likely it's been deleted, so do nothing
		return reconcile.Result{}, nil
	}

	if site.Id() == "" {
		site.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), site); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Create a requestHandler instance that will service this reconcile request
	rh := requestHandler{
		reconciler: r,
		app:        &fn.DrupalApplication{},
		env:        &fn.DrupalEnvironment{},
		site:       site,
		logger:     reqLogger,
	}

	// TODO - factor-out "site" below as part of FN-255
	// TODO - factor-out "reqLogger" below as part of FN-255

	isSiteMarkedToBeDeleted := site.GetDeletionTimestamp() != nil
	if isSiteMarkedToBeDeleted {
		// Clean up unowned and external Resources
		if err := r.finalizeDomainDbMapSecret(reqLogger, site); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.finalizeConfigMap(reqLogger, site); err != nil {
			return reconcile.Result{}, err
		}
		if err := rh.finalizeDatabase(); err != nil {
			return reconcile.Result{}, err
		}
		if requeue, err := r.finalizeDbPwdSecret(reqLogger, site); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}

		if requeue, err := r.removeFinalizer(reqLogger, site); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}
	}

	// Add finalizer for this CR.
	if site.GetDeletionTimestamp() == nil {
		if requeue, err := r.addFinalizer(reqLogger, site); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}
	}

	// Get parent environment
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: site.Namespace, Name: site.Spec.Environment}, rh.env)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Parent Environment doesn't exist", "Environment Name", rh.env.Name)
			// Delay the requeue rather than returning an error, to avoid exponential error backoff
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		} else {
			reqLogger.Error(err, "Failed to get Environment", "Environment Name", rh.env.Name)
			return reconcile.Result{}, err
		}
	}

	// Get super-parent application
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: rh.env.Spec.Application}, rh.app)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Parent Application doesn't exist", "Application Name", rh.app.Name)
			return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
		} else {
			reqLogger.Error(err, "Failed to get Application", "Application Name", rh.app.Name)
			return reconcile.Result{}, err
		}
	}

	requeue, err := rh.linkToEnvironment()
	if err != nil {
		reqLogger.Error(err, "Failed to link to parent Environment")
		return reconcile.Result{Requeue: requeue}, err
	}
	if requeue {
		return reconcile.Result{Requeue: requeue}, nil
	}

	if requeue, err := r.reconcileDbPwdSecret(reqLogger, site); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := r.reconcileDomainMap(reqLogger, site); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := rh.reconcileDomainDbMapSecret(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := rh.reconcileDatabase(); requeue || err != nil {
		if requeue {
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}
		return reconcile.Result{}, err
	}

	if requeue, err := rh.reconcileCronJobs(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := rh.reconcileJobs(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if err := r.updateIngress(reqLogger, site); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// addFinalizer adds a finalizer to the Site CR, to coordinate cleanup of the DB and non-owned subresources
func (r *ReconcileSite) addFinalizer(reqLogger logr.Logger, s *fn.Site) (requeue bool, err error) {
	if common.AddFinalizer(siteCleanupFinalizer, s) {
		return true, r.client.Update(context.TODO(), s)
	}
	return false, nil
}

func (r *ReconcileSite) removeFinalizer(reqLogger logr.Logger, s *fn.Site) (requeue bool, err error) {
	if common.RemoveFinalizer(siteCleanupFinalizer, s) {
		return true, r.client.Update(context.TODO(), s)
	}
	return false, nil
}

// linkToEnvironment sets the owner reference of the Site to the DrupalEnvironment it is hosted in, and
// updates Site labels with those from the environment.
func (rh *requestHandler) linkToEnvironment() (requeue bool, err error) {
	update, err := common.LinkToOwner(rh.env, rh.site, rh.reconciler.scheme)
	if err != nil {
		return false, err
	}
	if update {
		if err := rh.reconciler.client.Update(context.TODO(), rh.site); err != nil {
			return false, err
		}
	}

	return update, nil
}

func (rh *requestHandler) reconcileDatabase() (requeue bool, err error) {
	adminDB, err := common.GetAdminConnection(rh.reconciler.client)
	if err != nil {
		return false, err
	}
	defer func() {
		err = adminDB.Close()
		if err != nil {
			rh.logger.Error(err, "adminDB.Close() failed")
		}
	}()

	if err := adminDB.Ping(); err != nil {
		rh.logger.Error(err, "adminDB.Ping() failed")
		return true, nil
	}

	proxysqlAdminConn, err := common.GetProxySqlAdminConnection(rh.reconciler.client, rh.site.Namespace)
	if err != nil {
		return false, err
	}
	defer func() {
		err = proxysqlAdminConn.Close()
		if err != nil {
			rh.logger.Error(err, "proxysqlAdminConn.Close() failed")
		}
	}()

	if err := proxysqlAdminConn.Ping(); err != nil {
		rh.logger.Error(err, "proxysqlAdminConn.Ping() failed")
		return true, nil
	}

	siteDB, err := rh.getDB()
	if err != nil {
		return false, err
	}

	_, err = adminDB.Exec("CREATE DATABASE IF NOT EXISTS " + siteDB.Name)
	if err != nil {
		return false, err
	}

	if _, err := adminDB.Exec(fmt.Sprintf("CREATE USER '%s'@'%%'", siteDB.User)); err != nil {
		// 1396 is ERR_CANNOT_USER in mysql5.6. In this case, it means the user already
		// exists in the system and cannot be created again.  This is the only error we
		// are happy to see, so we just log that there is nothing to do and move on.  All
		// other errors are failure cases.
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1396 {
			return false, err
		}
		rh.logger.Info(fmt.Sprintf("User '%s' already exists.", siteDB.User))
	}

	if _, err := adminDB.Exec(fmt.Sprintf("SET PASSWORD FOR '%s'@'%%' = PASSWORD('%s')", siteDB.User, siteDB.Password)); err != nil {
		return false, err
	}

	_, err = adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'", siteDB.Name, siteDB.User))
	if err != nil {
		return false, err
	}

	_, err = adminDB.Exec(fmt.Sprintf("FLUSH PRIVILEGES"))
	if err != nil {
		return false, err
	}

	if _, err := proxysqlAdminConn.Exec(fmt.Sprintf(`INSERT INTO mysql_users(username,password,default_hostgroup) VALUES ('%s','%s',1)`, siteDB.User, siteDB.Password)); err != nil {
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1045 {
			return false, err
		}
		rh.logger.Info(fmt.Sprintf("User '%s' already exists.", siteDB.User))
	}

	_, err = proxysqlAdminConn.Exec(`LOAD MYSQL USERS TO RUNTIME`)
	if err != nil {
		return false, err
	}

	_, err = proxysqlAdminConn.Exec(`SAVE MYSQL USERS TO DISK`)
	if err != nil {
		return false, err
	}

	return false, nil
}

// reconcileDbPwdSecret creates the db pwd secret and adds a finalizer for it.
func (r *ReconcileSite) reconcileDbPwdSecret(reqLogger logr.Logger, s *fn.Site) (requeue bool, err error) {
	pwdSecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name + "-password"}, pwdSecret)
	if err != nil && errors.IsNotFound(err) {
		// generate password.
		password, err := common.RandPassword()
		if err != nil {
			return true, err
		}
		pwdSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.Name + "-password",
				Namespace: s.Namespace,

				// Adding a finalizer to the secret in order to prevent the secret from
				// getting deleted by processes like namespace deletion.
				Finalizers: []string{dbPwdSecretFinalizer},
			},
			StringData: map[string]string{
				"password": password,
			},
			Type: "Opaque",
		}
		r.associateResourceWithController(reqLogger, pwdSecret, s)

		if err := r.client.Create(context.TODO(), pwdSecret); err != nil {
			return false, err
		}
		return true, nil
	} else if err != nil && !errors.IsNotFound(err) {
		return false, err
	}
	return false, nil
}

func (r *ReconcileSite) reconcileDomainMap(reqLogger logr.Logger, s *fn.Site) (requeue bool, err error) {
	targetName := fn.DomainMapName
	targetNamespace := s.Namespace
	domainMap := &corev1.ConfigMap{}
	cmdata := ConfigMapData{}

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: targetName, Namespace: targetNamespace}, domainMap)
	if err != nil {
		return false, err
	}

	// ensure domainMap is up to date
	cmdata.Parse(domainMap.Data)
	if cmdata.EnsureDomainMapPresence(s) {
		reqLogger.Info(fmt.Sprintf("ConfigMap %s out of date. Updating...", fn.DomainMapName))
		domainMap.Data, err = cmdata.Write()
		if err != nil {
			return false, err
		}
		if err := r.client.Update(context.TODO(), domainMap); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (rh *requestHandler) reconcileDomainDbMapSecret() (requeue bool, err error) {
	targetName := fn.DomainMapName
	targetNamespace := rh.site.Namespace
	dbSecret := &corev1.Secret{}
	dbmap := SecretData{}

	var db common.Database
	if db, err = rh.getDB(); err != nil {
		return false, err
	}

	err = rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Name: targetName, Namespace: targetNamespace}, dbSecret)
	if err != nil {
		return false, err
	}

	// ensure db secret is up to date
	dbmap.Parse(dbSecret.Data)
	if dbmap.EnsureDBMapPresence(rh.site.Id(), db) {
		rh.logger.Info(fmt.Sprintf("Secret %s out of date. Updating...", fn.DomainMapName))
		dbSecret.StringData, err = dbmap.Write()
		if err != nil {
			return false, err
		}
		if err := rh.reconciler.client.Update(context.TODO(), dbSecret); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *ReconcileSite) updateIngress(reqLogger logr.Logger, s *fn.Site) error {
	targetName := s.Name
	targetNamespace := s.Namespace
	desiredIngAnnotations := map[string]string{
		"certmanager.k8s.io/cluster-issuer": s.IngressCertIssuer(),
		"kubernetes.io/ingress.class":       s.IngressClass(),
	}
	ing := &extv1b1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: targetName, Namespace: targetNamespace}, ing)
	if err != nil && errors.IsNotFound(err) {
		ing = &extv1b1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        targetName,
				Namespace:   targetNamespace,
				Labels:      s.ChildLabels(),
				Annotations: desiredIngAnnotations,
			},
			Spec: extv1b1.IngressSpec{
				Rules: s.IngressRules(),
				TLS:   s.IngressTLS(),
			},
		}
		r.associateResourceWithController(reqLogger, ing, s)

		reqLogger.Info("Creating new Ingress", "name", ing.Name, "namespace", ing.Namespace)
		if err := r.client.Create(context.TODO(), ing); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	// ensure ingress is up to date
	rules, tls, ingAnnotations := ing.Spec.Rules, ing.Spec.TLS, ing.ObjectMeta.Annotations
	desiredRules, desiredTLS := s.IngressRules(), s.IngressTLS()
	update := false
	if !reflect.DeepEqual(rules, desiredRules) {
		reqLogger.Info("Ingress rules out of date. Updating...")
		update = true
		ing.Spec.Rules = desiredRules
	}
	if !reflect.DeepEqual(tls, desiredTLS) {
		reqLogger.Info("Ingress tls out of date. Updating...")
		update = true
		ing.Spec.TLS = desiredTLS
	}
	for anno := range desiredIngAnnotations {
		if ingAnnotations[anno] != desiredIngAnnotations[anno] {
			reqLogger.Info("Ingress annotation out of date. Updating...")
			update = true
			ing.ObjectMeta.Annotations[anno] = desiredIngAnnotations[anno]
		}
	}
	if update {
		if err := r.client.Update(context.TODO(), ing); err != nil {
			return err
		}
	}

	return nil
}

// Removes this site's section from the configmap while leaving the rest of the configmap intact
func (r *ReconcileSite) finalizeConfigMap(reqLogger logr.Logger, s *fn.Site) error {
	reqLogger.Info("Cleaning up ConfigMap " + fn.DomainMapName)
	domainMap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: fn.DomainMapName, Namespace: s.Namespace}, domainMap)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(fmt.Sprintf("ConfigMap %s not found, nothing to do.", fn.DomainMapName))
		return nil
	} else if err != nil {
		return err
	}

	cmdata := ConfigMapData{}
	cmdata.Parse(domainMap.Data)
	delete(cmdata, s.Id())
	domainMap.Data, err = cmdata.Write()
	if err != nil {
		return err
	}

	if err := r.client.Update(context.TODO(), domainMap); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(fmt.Sprintf("ConfigMap %s not found, nothing to do.", fn.DomainMapName))
			return nil
		}
		return err
	}
	return nil
}

// Removes this site's section from the secret while leaving the rest of the secret intact
func (r *ReconcileSite) finalizeDomainDbMapSecret(reqLogger logr.Logger, s *fn.Site) error {
	dbSecret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: fn.DomainMapName, Namespace: s.Namespace}, dbSecret)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(fmt.Sprintf("Secret %s not found, nothing to do.", fn.DomainMapName))
		return nil
	} else if err != nil {
		return err
	}

	dbmap := SecretData{}
	dbmap.Parse(dbSecret.Data)
	delete(dbmap, s.Id())
	dbSecret.StringData, err = dbmap.Write()
	if err != nil {
		return err
	}

	if err := r.client.Update(context.TODO(), dbSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info(fmt.Sprintf("Secret %s not found, nothing to do.", fn.DomainMapName))
			return nil
		}
		return err
	}
	return nil
}

// For now, this drops the database and removes the user from the cluster.
// Eventually, this could be used to take a final backup or something similar
func (rh *requestHandler) finalizeDatabase() error {
	adminDB, err := common.GetAdminConnection(rh.reconciler.client)
	if err != nil {
		return err
	}
	defer adminDB.Close()
	if err := adminDB.Ping(); err != nil {
		return err
	}

	siteDB, err := rh.getDB()
	if err != nil {
		return err
	}

	// Cleanup admin DB
	_, err = adminDB.Exec("DROP DATABASE IF EXISTS " + siteDB.Name)
	if err != nil {
		return err
	}

	if _, err = adminDB.Exec(fmt.Sprintf("DROP USER '%s'@'%%'", siteDB.User)); err != nil {
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1396 {
			return err
		}
		rh.logger.Info(fmt.Sprintf("User '%s' already dropped.", siteDB.User))
	}

	// Cleanup ProxySQL
	proxysqlAdminConn, err := common.GetProxySqlAdminConnection(rh.reconciler.client, rh.site.Namespace)
	if err != nil {
		// Service is not found
		if errors.IsNotFound(err) {
			rh.logger.Info("ProxySQL service not found, nothing to do.")
			return nil
		}
		return err
	}

	defer proxysqlAdminConn.Close()
	if err := proxysqlAdminConn.Ping(); err != nil {
		return err
	}

	// ERROR 1045 - error thrown by proxysql for almost all cases
	if _, err := proxysqlAdminConn.Exec(fmt.Sprintf(`DELETE FROM mysql_users WHERE username='%s'`, siteDB.User)); err != nil {
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1045 {
			return err
		}
		rh.logger.Info(fmt.Sprintf("User '%s' deleted", siteDB.User))
	}

	_, err = proxysqlAdminConn.Exec(`LOAD MYSQL USERS TO RUNTIME`)
	if err != nil {
		return err
	}

	_, err = proxysqlAdminConn.Exec(`SAVE MYSQL USERS TO DISK`)
	if err != nil {
		return err
	}

	return nil
}

// Removes the database password secret finalizer.
func (r *ReconcileSite) finalizeDbPwdSecret(reqLogger logr.Logger, s *fn.Site) (requeue bool, err error) {
	dbPwdSecret := &corev1.Secret{}
	pwdNameSpace := types.NamespacedName{Namespace: s.Namespace, Name: s.Name + "-password"}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name + "-password"}, dbPwdSecret)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(fmt.Sprintf("Database password Secret on %s not found, nothing to do.", pwdNameSpace))
		return false, nil
	}

	removed := common.RemoveFinalizer(dbPwdSecretFinalizer, dbPwdSecret)
	if removed {
		reqLogger.Info("Removing finalizer")
		if err := r.client.Update(context.TODO(), dbPwdSecret); err != nil {
			reqLogger.Info(fmt.Sprintf("failed to remove the database password secret finalizer on %s .", pwdNameSpace))
			return false, err
		}
		return false, nil
	}
	return false, fmt.Errorf("failed to remove the db password secret")
}

func (r ReconcileSite) associateResourceWithController(reqLogger logr.Logger, o metav1.Object, s *fn.Site) {
	err := controllerutil.SetControllerReference(s, o, r.scheme)
	if err != nil {
		reqLogger.Error(err, "Failed to set controller as owner", "Resource", o)
	}
}
