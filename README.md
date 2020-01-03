# muminio

## Introduction

Goals of this project:
* provide a better "glue" between https://min.io/ and apps deployed to Kubernetes
* applications shouldn't be aware that Minio is running in native or in gateway mode
* applications can create buckets declaratively via customer resources
* applications can control the secrets used to access the buckets
* provide isolation between applications, app 1 should not have access to write to a bucket of app 2

Implementation is done via https://github.com/operator-framework/operator-sdk which takes care of all the boilerplate code.

## Installation

The operator and shared Minio instance will be deployed to a new namespace "minio-system". They will both share the same Secret that holds the master access keys to Minio.

### Create namespace
```
kubectl apply -f deploy/01-namespace.yaml
```

### Deploy Minio
There are multiple options to do this, which is kind of the point of the whole project. Applications shouldn't be aware of which Minio mode is deployed (standalone, distributed or gateway). The same application should work on any Minio installation without modifications.

#### Minio Azure Gateway
First, you have to create a storage account in Azure and then create a Secret that holds the account credentials. Create the following as 03-minio-secret.yaml: 
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: minio-master-secret
  namespace: minio-system
type: Opaque
data:
  accessKey: TUlOSU9fTUFTVEVSX0FDQ0VTU19LRVk= # Replace with storage account name
  secretKey: ZjZmYWExODU3ZGNlYzc2M2JmYjE1MzQ3NTlhZjNjYTNkOWQ3MTBhZDliM2Y2NDY4OTA5ZDQzNjBmNDhhMjM1OA== # Replace with secret key for storage account
```
The above file name is also in the .gitignore file to prevent accidential check-in of credentials

The Minio Azure gateway needs an etcd cluster to store additional user credentials, which is required by muminio. 

Then, apply the files in the correct order. You have to pause a bit after the etcd operator, because the CRD is only created when the operator is up and running and if the CRD is not there, the deployment of 04-minio.yaml will fail.

```
kubectl apply -f deploy/minio-azure-gateway/02-etcd-operator.yaml
sleep 20
kubectl apply -f deploy/minio-azure-gateway/03-minio-secret.yaml
kubectl apply -f deploy/minio-azure-gateway/04-minio.yaml
```

Minio is now available as a LoadBalancer service. Try to open it in a browser, e.g. with
```
kubectl port-forward service/minio 9000 -n minio-system
```
and then open http://localhost:9000 . You have to provide the credentials for the Azure storage account.

#### Minio distributed 
An example deployment is in `deploy/minio-distributed/`. You can adapt that, or use any other means to deploy Minio (e.g. Helm chart). After installation, Minio should be available as a service `minio` in namespace `minio-system`. 

The example assumes a hostpath provisioner which is typically used in small clusters like minikube. Please adapt as necessary. 

```
kubectl apply -f deploy/minio-distributed/
```

### Deploy Operator
We have to deploy the custom resource definition and the operator itself:

```
kubectl apply -f deploy/02-muminio-crd.yaml
kubectl apply -f deploy/03-muminio-operator.yaml
```

If all pods come up without errors, the installation should now be complete. 

## Sample
A simple example can be found in `deploy/examples/basic`. Deploy it with
```
kubectl apply -f deploy/examples/basic/basic.yaml
```

This will create a new namespace and a custom resource:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: muminio-example-basic
---
apiVersion: muminio.benien.dev/v1alpha1
kind: MuminioBucket
metadata:
  name: bucket-basic-example
  namespace: muminio-example-basic
spec:
  secretName: muminio-example-basic-secret
---
kind: Secret
apiVersion: v1
metadata:
  name: muminio-example-basic-secret
  namespace: muminio-example-basic
data:
  accessKey: TUlOSU9fQUNDRVNTX0tFWV8xMjM0NQ==
  secretKey: NzRlZmZhOGE4ZTliNzQ3NGRlNjA2YTgxODE4M2MxODhkMmJmMTU4Yjg3OTBkNjc4M2QwMmExMDBmNThjODJkMg==
type: Opaque
```
The custom resource of type MuminioBucket tells the Muminio operator to create a bucket called `bucket-basic-example` and make it available with the credentials specified in the Kubernetes secret `muminio-example-basic-secret`

The credentials in plain text are:
```
accessKey: MINIO_ACCESS_KEY_12345
secretKey: 74effa8a8e9b7474de606a818183c188d2bf158b8790d6783d02a100f58c82d2
```

You can also use them in the Minio browser (see above). These credentials only have access to a single bucket, this provides isolation between applications. 

Next comes a Deployment that continuously writes and reads objects from Minio, implemented with the AWS SDK (Boto3) in Python. The source code is the same directory. 

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: do-stuff-with-minio
  namespace: muminio-example-basic
spec:
  replicas: 1
  selector:
    matchLabels:
      name: do-stuff-with-minio
  template:
    metadata:
      labels:
        name: do-stuff-with-minio
    spec:
      containers:
        - name: muminio
          image: keaphyra/muminio-basic-example:latest
          env:
            - name: MINIO_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: muminio-example-basic-secret
                  key: accessKey
            - name: MINIO_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: muminio-example-basic-secret
                  key: secretKey
            - name: MINIO_URL
              value: minio.minio-system:9000
            - name: MINIO_SECURE
              value: "false"
```

Look at the logs of the deployed pods to see if they can successfully read and write to Minio. You can also look at the Minio browser or the Azure Storage browser (if Minio is deployed in gateway mode)


























```
kubectl apply -f deploy/minio-azure-gateway/
```




* Deploy CRD
* Deploy Operator (TODO)
* Deploy example application (TODO)

## TODO list

* Operator metrics
* Show failure to connect to minio endpoint (liveness probe?)

