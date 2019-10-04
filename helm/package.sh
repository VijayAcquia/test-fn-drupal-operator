#!/usr/bin/env bash

chartName='fn-drupal-operator'
bucketName='kpoc-helm'
set -e

for i in "$@"
do
case $i in
    -p|--push)
    PUSH=true
    ;;
    -f|--force)
    FORCE=true
    ;;
esac
done

if [ ! -d "./${chartName}/templates/" ]
then
  mkdir -p ./${chartName}/templates/
fi

rm -rf ./${chartName}/templates/*
cp ../deploy/*.yaml ./${chartName}/templates/
cp ../deploy/crds/*_crd.yaml ./${chartName}/templates/

helm package ./${chartName}

# Capturing the Chart Name
version=$(grep 'version:' ./${chartName}/Chart.yaml | tail -n1 | awk '{ print $2}')
chartFileName="${chartName}-${version}.tgz"
echo ${chartFileName};

# You can pus the chart to repository by providing either of following option --push or -p
# If chart already exists then you can override it by providing --force or -f option.
force=""
if [ "$FORCE" = true ]
then
  force="--force"
fi

if [ "$PUSH" = true ]
then
  helm s3 push $force $chartFileName kpoc
fi
