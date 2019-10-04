#!/usr/bin/env sh
set -ex

OLD_POD=$(kubectl get -n services pod -oname | grep fn-drupal-operator-)
kubectl delete -n services "${OLD_POD}"

sleep 2

NEW_POD=$(kubectl get -n services pod -oname | grep fn-drupal-operator-)
set +ex
kubectl logs -f -n services "${NEW_POD}" | while read -r line ; do
  echo $line | jq -rcRC '. as $raw | try fromjson catch $raw'
  echo
done
