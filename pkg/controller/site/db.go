package site

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/acquia/fn-drupal-operator/pkg/common"
)

const proxySqlAdminHost = "proxysql"
const proxySqlAdminPort = "6033"

func (rh *requestHandler) getDB() (common.Database, error) {
	s := rh.site
	var passwd string
	var err error

	if passwd, err = rh.getPassword(); err != nil {
		return common.Database{}, err
	}

	return common.Database{
		Host:     proxySqlAdminHost,
		Port:     proxySqlAdminPort,
		Name:     s.DatabaseName(),
		User:     s.DatabaseUser(),
		Password: passwd,
	}, nil
}

// getPassword returns the database password secret.
func (rh *requestHandler) getPassword() (string, error) {
	s := rh.site
	pwdSecret := &corev1.Secret{}
	if err := rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name + "-password"}, pwdSecret); err != nil {
		return "", err
	}
	return string(pwdSecret.Data["password"]), nil
}
