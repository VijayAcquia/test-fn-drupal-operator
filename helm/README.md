# Packaging of operators

##### Prequisites
- helm s3 plugin must be installed. You can install it by using below command: 
  ```helm plugin install https://github.com/hypnoglow/helm-s3.git```

##### Shell Scripts
- `package.sh`
It will copy the .yaml to chart's templates folder, package the chart and push it into the helm repository.
  
  - `-p` or `--push` option will be used to push the package to the repository.
  - `-f` or `--force` option will be used to forcefully overwrite the package of the same version.
