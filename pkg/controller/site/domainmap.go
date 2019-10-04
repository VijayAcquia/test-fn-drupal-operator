package site

import (
	"encoding/json"
	"reflect"

	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	common "github.com/acquia/fn-drupal-operator/pkg/common"
)

type ConfigMapData map[fn.SiteId]fn.DomainMap
type SecretData map[fn.SiteId]common.Database

const mapKey = "dbconfig.json"

// ConfigMap-specific functions

// Parses the data from a configmap into a useful structure (ConfigMapData)
// We abstract away the configmap key since it is not useful to this compoment
func (cmdata *ConfigMapData) Parse(data map[string]string) error {
	return json.Unmarshal([]byte(data[mapKey]), cmdata)
}

// Write the ConfigMapData object back to the format that a configmap expects.
// This is Primarily for abstracting away the usage of the configmap key,
// as it is required for kubernetes and the pod that mounts it, but useless to us.
func (cmdata ConfigMapData) Write() (map[string]string, error) {
	data, err := json.Marshal(cmdata)
	return map[string]string{mapKey: string(data)}, err
}

// This ConfigMap is shared and edited by multiple Sites, so each site has a
// location within it that only it will write to: the value at its Id.
// This function then ensures that the value at its Id is reconciled with
// what the Site dictates.
func (cmdata *ConfigMapData) EnsureDomainMapPresence(s *fn.Site) bool {
	id := s.Id()
	desiredMap := s.DomainMap()
	domains, ok := (*cmdata)[id]
	if ok && reflect.DeepEqual(desiredMap, domains) {
		return false
	}
	(*cmdata)[id] = desiredMap
	return true
}

// Secret-specific functions

// Parses the data from a secret into a useful structure (SecretData)
// We abstract away the secret key since it is not useful to this compoment
func (dbmap *SecretData) Parse(data map[string][]byte) error {
	return json.Unmarshal(data[mapKey], dbmap)
}

// Write the SecretData object back to the format that a secret expects.
// This is Primarily for abstracting away the usage of the secret key,
// as it is required for kubernetes and the pod that mounts it, but useless to us.
func (dbmap SecretData) Write() (map[string]string, error) {
	data, err := json.Marshal(dbmap)
	return map[string]string{mapKey: string(data)}, err
}

// This Secret is shared and edited by multiple Sites, so each site has a
// location within it that only it will write to: the value at its Id.
// This function then ensures that the value at its Id is reconciled with
// what the Site dictates.
func (dbmap *SecretData) EnsureDBMapPresence(id fn.SiteId, db common.Database) bool {
	domains, ok := (*dbmap)[id]
	if ok && reflect.DeepEqual(db, domains) {
		return false
	}
	(*dbmap)[id] = db
	return true
}
