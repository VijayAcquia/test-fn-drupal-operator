package common

import (
	"context"
	"crypto/rand"
	"database/sql"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Database struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Name     string `json:"database"`
	User     string `json:"user"`
	Password string `json:"pass"`
}

func RandPassword() (string, error) {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789" +
		`~!@#$%^&*()_+-=[]{}:,./?`)
	length := 12
	var b strings.Builder
	for i := 0; i < length; i++ {
		indx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars)))) // NOTE - crypto/rand library is not FIPS 140-2 validated, and never will be: see https://github.com/golang/go/issues/11658#issuecomment-120448974
		if err != nil {
			return "", err
		}
		b.WriteRune(chars[indx.Int64()])
	}
	return b.String(), nil
}

func GetAdminDB(c client.Client) (Database, error) {
	dbAdminSecret := &corev1.Secret{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: "default-cluster-creds", Namespace: "default"}, dbAdminSecret); err != nil {
		return Database{}, err
	}

	db := Database{}
	data := dbAdminSecret.Data
	if user := os.Getenv("DB_USER_OVERRIDE"); user != "" {
		db.User = user
	} else {
		db.User = string(data["username"])

	}
	if passwd := os.Getenv("DB_PASSWORD_OVERRIDE"); passwd != "" {
		db.Password = passwd
	} else {
		db.Password = string(data["password"])

	}
	if host := os.Getenv("DB_HOST_OVERRIDE"); host != "" {
		db.Host = host
	} else {
		db.Host = string(data["host"])
	}
	if port := os.Getenv("DB_PORT_OVERRIDE"); port != "" {
		db.Port = port
	} else {
		db.Port = string(data["port"])
	}

	return db, nil
}

func (db Database) GetConnection() (*sql.DB, error) {
	config := mysql.NewConfig()
	config.User = db.User
	config.Passwd = db.Password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(db.Host, db.Port)
	config.Timeout = time.Second * 5

	conn, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return nil, err
	}
	conn.SetConnMaxLifetime(time.Second * 10)
	return conn, err
}

func GetAdminConnection(c client.Client) (*sql.DB, error) {
	db, err := GetAdminDB(c)
	if err != nil {
		return nil, err
	}
	return db.GetConnection()
}

func GetProxySqlAdminConnection(c client.Client, namespace string) (*sql.DB, error) {
	// TODO - check if 'proxysql' Deployment is Ready before attempting to connect

	found := &corev1.Service{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: "proxysql", Namespace: namespace}, found)
	if err != nil {
		return nil, err
	}

	proxySqlDb := Database{
		Host:     found.Spec.ClusterIP,
		Name:     "main",
		User:     "proxysql-admin",
		Password: "adminpassw0rd", // FIXME !!! - https://backlog.acquia.com/browse/FN-240
		Port:     "6032",
	}

	return proxySqlDb.GetConnection()
}
