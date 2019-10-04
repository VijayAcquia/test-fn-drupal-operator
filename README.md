# Drupal Application Operator

This application is built utilizing the Operator SDK, and provides several controllers which orchestrate the Kubernetes
hosting infrastructure necessary to serve Drupal applications:

## Controllers

### Drupal Environment Controller

The `DrupalEnvironment` Controller manages Kubernetes resources needed to provide a hosted Drupal environment (as in
"dev", "stage", "prod", etc.). Its Custom Resource Definition is contained in `deploy/crds/fnresources_v1alpha1_drupalenvironment_crd.yaml`.

The `DrupalEnvironment` Controller creates `Deployment`s and `Service`s for "Drupal" pods containing Apache and PHP-FPM. 
`ConfigMap`s and `Secret`s are created to hold configuration for Apache, PHP, and PHP-FPM. A `HorizontalPodAutoscaler`
is created to automatically scale the "Drupal" `Deployment` to handle fluctuations in load.

A ProxySQL `Deployment` is also created to serve as an intermediary between the Drupal `Pod`s and the external database
cluster. A `ConfigMap` and `Secret` are created to hold the initial configuration and credentials needed for ProxySQL to
perform its function. ProxySQL is used to manage a pool of MySQL connections which are reused, reducing the number of
new connections that are created with the (Aurora) external DB cluster, which works around and issue with Aurora's
auto-scaling mechanism not scaling up enough to accept this many new connections.

### Site Controller

The `Site` Controller manages Kubernetes resources needed to serve a Drupal site from within a given Drupal environment.
Its Custom Resource Definition is contained in `deploy/crds/fnresources_v1alpha1_site_crd.yaml`.

The `Site` Controller creates an `Ingress` for the site, and configures the "domain map" `ConfigMap` and `Secret`, which
contain the Drupal Multisite mapping and DB credentials, respectively. It also manages the ProxySQL connection to the site's
DB.

`Job`s to be run can be added to a `Site` Custom Resource as annotations. The `Site` Controller will manage the running
of these `Job`s. `CronJob`s to run periodically can also be added to a `Site` by using the `Site.spec.crons` field. See 
`deploy/crds/fnresources_v1alpha1_site_cr.yaml` for examples of both of these.

## Namespaces in this file
Many of the example commands in this file omit the --namespace or -n option.
This is enabled by first using the `kubens` command:

```bash
kubens services
```

## Build

1. Install operator-sdk
   - MacOS `brew install operator-sdk`
1. Clone repo
1. Build and push a Docker image containing the operator:

    ```bash
    your_image_tag=SOMETHING UNIQUE
    operator-sdk build 881217801864.dkr.ecr.us-east-1.amazonaws.com/drupal-operator:${your_image_tag} \
      && docker push 881217801864.dkr.ecr.us-east-1.amazonaws.com/drupal-operator:${your_image_tag}
    ```

## Deployment on Cluster

1. Generate a local Helm chart:

    ```bash
    cd helm
    ./package.sh
    ```

1. Deploy the Helm chart:

    ```bash
    helm install --name fn-drupal-operator --namespace services ./fn-drupal-operator \
      --set image.tag=${your_image_tag}
    ```

    If you want the Operator to only watch a specific Namespace, that can be specified by adding the command line option:

    ```bash
    --set watchNamespace=${namespace_to_watch}
    ```

1. Monitor the logs with:

    ```bash
    kubectl logs -f -n services fn-drupal-operator-XXXXXXXXXX-XXXXX
    ```

    or, if you have `jq` installed, you can pretty-print them:

    ```bash
    kubectl logs -f -n services fn-drupal-operator-XXXXXXXXXX-XXXXX \
      | while read -r line ; do echo ${line} | jq -crRC '.' ; echo "" ; done
    ```

## Local Development Using a Remote Kubernetes Cluster

You can compile and run your local operator code by running:

```bash
operator-sdk up local --namespace "" --kubeconfig $HOME/.kube/your_kubeconfig_file
```

The operator will run locally, but perform all API operations on the cluster specified in the given kubeconfig file. The 
main advantages of this are 1) not needing to build a Docker image on every run, and 2) not needing to kill the operator
Pod on the cluster to relaunch your operator build with the new Docker image.

It should be possible for multiple developers to run the operator on the same remote cluster by specifying different
Namespace values on the `--namespace` parameter. That way, each operator will only watch for `DrupalEnvironment` and `Site` 
Custom Resources in the given namespace, and won't interfere with each other. (Specifying `--namespace ""` causes the
operator to watch Custom Resources on all Namespaces on the cluster.)

### Pretty-printing Operator Logs with `jq`

If you have the `jq` CLI utility installed locally, you can (mostly) pretty-print the JSON-based log output that comes from
the operator. For a deployment of the Operator on a remote cluster, use a command similar to:

```bash
kubectl logs -f fn-drupal-operator-xxxxxxxxxx-xxxxx | while read -r line ; do \
  echo $line | jq -rcRC '. as $raw | try fromjson catch $raw' ; echo "" ; done
```

Or, if running the Operator locally, try:

```bash
operator-sdk up local --namespace "" --kubeconfig $HOME/.kube/your_kubeconfig_file 2>&1 \
  | while read -r line ; do echo ${line} | jq -crRC '. as $raw | try fromjson catch $raw' \
  ; echo "" ; done
```

## Packaging for Production/Staging Release (preliminary)

1. Build Operator image and push to next version tag

    ```bash
    export NEW_VERSION=v1.1.0
    operator-sdk build 881217801864.dkr.ecr.us-east-1.amazonaws.com/drupal-operator:$NEW_VERSION \
      && docker push 881217801864.dkr.ecr.us-east-1.amazonaws.com/drupal-operator:NEW_VERSION
    ```

1. Update Helm chart version by editing `helm/fn-drupal-operator/Chart.yaml` and changing `version:` and `appVersion:`

1. Package Helm chart and push to S3 chart repo:

    ```bash
    cd helm
    ./package.sh --push
    ```

### Releasing to Production/Staging (preliminary)

1. Upgrade Helm chart on production/staging cluster

    ```bash
    helm upgrade fn-drupal-operator kpoc/fn-drupal-operator
    ```

## Local development ## 

A running local cluster following the steps available on kpoc repository is required for local development: 
* [kpoc.README](https://github.com/acquia/kpoc/blob/master/local/README.md)

### Prerequisites 
  
1. Download and install operator-sdk: [operator-sdk](https://github.com/operator-framework/operator-sdk)

### Setup Operators

Run the following commands on `fn-drupal-operator` to set up the operators:
             
 ```
 cd helm
 ./package.sh
 helm install --name fn-drupal-operator --namespace services ./helm/fn-drupal-operator --set useDynamicProvisioning="true"
 ```
             
Once the custom resource definitions for drupal environment and  site are installed, run the following commands to create a drupal environment and site:
  
* `kubectl apply -f deploy/crds/fnresources_v1alpha1_drupalenvironment_cr.yaml`
* `kubectl apply -f deploy/crds/fnresources_v1alpha1_site_cr.yaml`


### Working with controllers

* Find the image name currently deployed:

In order to get the controller changes up and running on the local cluster, run the following command to get the image name:

```
kubectl get deploy fn-drupal-operator -o jsonpath="{.spec.template.spec.containers[0].image}"
```

If above did not work, run below and find the image tag and copy the image name:
 
```
kubectl get deploy fn-drupal-operator -o yaml"
``` 


* Build a new image using your local code:

```
operator-sdk build [image name from the above step]:[your custom name for your image]
```


* Push the image to dockerhub:

```
docker push [image name from the above step]:[your custom name for your image]
```

* Set the image on the fn-drupal-operator deployment:
 
```
kubectl set image deployment fn-drupal-operator fn-drupal-operator=[image name from the above step]:[your custom name for your image] -n services
```


### Verifying the resources

Run the following to get your pods/services or any other custom resource to see the changes up and running:

To get all the pods:

`kubectl get po -A`


To get all the services:

`kubectl get svc -A`

To get drupal environments:

`kubectl get drenv -A`

To get the sites:

`kubectl get drenv -A`


### Troubleshooting

## Mysql

Find the mysql connection info in the yaml file located in local directory in kpoc repo.
In order to connect to mysql locally on your machine you need to port forward the mysql service traffic to your local 3306 port:
 
`-> kubectl port-forward svc/mysql 33306:3306`

`-> mysql -uroot -padmin -h127.0.0.1 -P33306`

## Browse the site

In order to visit the site you just created, you can port forward varnish to the port you want your site to listen to:
 
`-> kubectl port-forward svc/varnish 8000:80` 

The site should be available to access at:

`http://127.0.0.1:8000`
