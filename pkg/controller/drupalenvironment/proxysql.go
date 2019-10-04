package drupalenvironment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

var proxySQLConfig string = `datadir="/var/lib/proxysql"
admin_variables=
{
        admin_credentials="proxysql-admin:adminpassw0rd"
        mysql_ifaces="0.0.0.0:6032"
        refresh_interval=2000
}
mysql_variables=
{
        threads=4
        max_connections=2048
        default_query_delay=0
        default_query_timeout=36000000
        have_compress=true
        poll_timeout=2000
        interfaces="0.0.0.0:6033;/tmp/proxysql.sock"
        default_schema="information_schema"
        stacksize=1048576
        server_version="5.1.30"
        connect_timeout_server=10000
        monitor_history=60000
        monitor_connect_interval=2000
        monitor_ping_interval=2000
        ping_interval_server_msec=10000
        ping_timeout_server=200
        commands_stats=true
        sessions_sort=true
}`

func (rh *requestHandler) reconcileProxySQL() (requeue bool, err error) {
	requeue, err = rh.reconcileConfigMap("proxysql-cnf", map[string]string{
		"proxysql.cnf": proxySQLConfig,
	})

	if err != nil || requeue {
		return requeue, err
	}

	r := rh.reconciler
	name := "proxysql"

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, dep, func(existing runtime.Object) error {
		realDEP := existing.(*appsv1.Deployment)
		desired := rh.proxysqlDeployment(name)

		if realDEP.CreationTimestamp.IsZero() {
			desired.DeepCopyInto(realDEP)
			rh.associateResourceWithController(realDEP)
			return nil
		}
		realDEP.Spec.Replicas = desired.Spec.Replicas
		realDEP.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes

		realContainer := &realDEP.Spec.Template.Spec.Containers[0]
		desiredContainer := &desired.Spec.Template.Spec.Containers[0]
		realContainer.Resources = desiredContainer.Resources
		realContainer.Image = desiredContainer.Image
		realContainer.VolumeMounts = desiredContainer.VolumeMounts

		return nil
	})

	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Successfully reconciled ProxySQL", "operation", op)
		return true, nil
	}

	return false, nil
}

func (rh *requestHandler) reconcileProxySQLPVC() (requeue bool, err error) {
	r := rh.reconciler

	pvc := rh.proxysqlPVC()

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, pvc, func(existing runtime.Object) error {
		realPVC := existing.(*v1.PersistentVolumeClaim)
		desired := rh.proxysqlPVC()

		if realPVC.CreationTimestamp.IsZero() {
			desired.DeepCopyInto(realPVC)
			rh.associateResourceWithController(realPVC)
			return nil
		}

		return nil
	})

	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Successfully reconciled ProxySQL PVC", "operation", op)
		return true, nil
	}

	return false, nil
}

func (rh *requestHandler) reconcileProxysqlService() (requeue bool, err error) {
	r := rh.reconciler
	name := "proxysql"

	svc := rh.proxysqlService(name)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, svc, func(existing runtime.Object) error {
		realSVC := existing.(*v1.Service)
		desired := rh.proxysqlService(name)

		if realSVC.CreationTimestamp.IsZero() {
			desired.DeepCopyInto(realSVC)
			rh.associateResourceWithController(realSVC)
			return nil
		}
		realSVC.Spec.Ports = desired.Spec.Ports
		realSVC.Spec.Selector = desired.Spec.Selector

		return nil
	})

	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Successfully reconciled ProxySQL Service", "operation", op)
		return true, nil
	}

	return false, nil
}

func (rh *requestHandler) reconcileProxysqlServerConfig() (requeue bool, err error) {
	clusterDbAdmin, err := common.GetAdminDB(rh.reconciler.client)
	if err != nil {
		rh.logger.Error(err, "GetAdminDB() failed")
		return false, err
	}

	proxySqlAdmin, err := common.GetProxySqlAdminConnection(rh.reconciler.client, rh.namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		rh.logger.Error(err, "GetProxySqlAdminConnection() failed")
		return true, nil
	}

	defer func() {
		err = proxySqlAdmin.Close()
		if err != nil {
			rh.logger.Error(err, "Close() failed")
		}
	}()

	if err := proxySqlAdmin.Ping(); err != nil {
		rh.logger.Error(err, "proxySqlAdmin.Ping() failed")
		return true, nil
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM mysql_servers WHERE hostname='%s'`, clusterDbAdmin.Host)
	row := proxySqlAdmin.QueryRow(query)
	var numRows int
	err = row.Scan(&numRows)

	if err != nil {
		rh.logger.Error(err, "Query failed", "Query", query)
		return false, err
	}

	if numRows == 0 {
		query = fmt.Sprintf(`INSERT INTO mysql_servers(hostgroup_id,hostname,port) VALUES (1,'%s',%s)`, clusterDbAdmin.Host, clusterDbAdmin.Port)
		_, err = proxySqlAdmin.Exec(query)
		if err != nil {
			rh.logger.Error(err, "Query failed", "Query", query)
			return false, err
		}
	}

	_, err = proxySqlAdmin.Exec(fmt.Sprintf(`UPDATE global_variables SET variable_value='%s' WHERE variable_name='mysql-monitor_password'`, clusterDbAdmin.Password))
	if err != nil {
		rh.logger.Error(err, "Query failed setting monitor password")
		return false, err
	}

	queries := []string{
		fmt.Sprintf(`UPDATE global_variables SET variable_value='%s' WHERE variable_name='mysql-monitor_username'`, clusterDbAdmin.User),
		fmt.Sprintf(`UPDATE global_variables SET variable_value='2000' WHERE variable_name IN ('mysql-monitor_connect_interval','mysql-monitor_ping_interval','mysql-monitor_read_only_interval')`),
		`LOAD MYSQL VARIABLES TO RUNTIME`,
		`SAVE MYSQL VARIABLES TO DISK`,
		`LOAD MYSQL SERVERS TO RUNTIME`,
		`SAVE MYSQL SERVERS TO DISK`,
	}

	for _, query = range queries {
		_, err = proxySqlAdmin.Exec(query)
		if err != nil {
			rh.logger.Error(err, "Query failed", "Query", query)
			return false, err
		}
	}

	return false, nil
}

func (rh *requestHandler) proxysqlService(name string) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "proxysql-mysql",
					Port:       6033,
					TargetPort: intstr.IntOrString{IntVal: 6033},
					Protocol:   "TCP",
				},
				{
					Name:       "proxysql-admin",
					Port:       6032,
					TargetPort: intstr.IntOrString{IntVal: 6032},
					Protocol:   "TCP",
				},
			},
			Selector: labelsForProxySQLDeployment(rh.env),
		},
	}
	return svc
}

func (rh *requestHandler) proxysqlPVC() *v1.PersistentVolumeClaim {
	storageclass := "gp2"
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxysql-data",
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &storageclass,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("128Mi"),
				},
			},
		},
	}
}

func (rh *requestHandler) proxysqlDeployment(name string) *appsv1.Deployment {
	accessMode := int32(420)

	proxySQLConfigMap := v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: "proxysql-cnf",
		},
		DefaultMode: &accessMode,
	}

	proxySQLDisk := v1.VolumeSource{
		PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
			ClaimName: "proxysql-data",
		},
	}

	ls := labelsForProxySQLDeployment(rh.env)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rh.env.Spec.ProxySQL.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"function": "workers",
					},
					Containers: []v1.Container{
						{
							Name:            "proxysql",
							Image:           "severalnines/proxysql:" + rh.env.Spec.ProxySQL.Tag,
							ImagePullPolicy: v1.PullIfNotPresent,
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 6033,
									Name:          "proxysql-mysql",
								},
								{
									ContainerPort: 6032,
									Name:          "proxysql-admin",
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    rh.env.Spec.ProxySQL.Cpu.Request,
									v1.ResourceMemory: rh.env.Spec.ProxySQL.Memory.Request,
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    rh.env.Spec.ProxySQL.Cpu.Limit,
									v1.ResourceMemory: rh.env.Spec.ProxySQL.Memory.Limit,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "proxysql-config",
									MountPath: "/etc/proxysql.cnf",
									SubPath:   "proxysql.cnf",
								},
								{
									Name:      "proxysql-disk",
									MountPath: "/var/lib/proxysql",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name:         "proxysql-config",
							VolumeSource: v1.VolumeSource{ConfigMap: &proxySQLConfigMap},
						},
						{
							Name:         "proxysql-disk",
							VolumeSource: proxySQLDisk,
						},
					},
				},
			},
		},
	}
	return dep
}

func labelsForProxySQLDeployment(drupalenv *fnv1alpha1.DrupalEnvironment) map[string]string {
	labels := drupalenv.ChildLabels()
	labels["app"] = "proxysql"
	return labels
}
