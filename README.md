# muminio

## Introduction

Goals of this project:
* provide a better "glue" between https://min.io/ and apps deployed to Kubernetes
* applications shouldn't be aware that Minio is running in native or in gateway mode
* applications can create buckets declaratively via customer resources
* applications can control the secrets used to access the buckets
* provide isolation between applications, app 1 should not have access to write to a bucket of app 2

Implementation is done via https://github.com/operator-framework/operator-sdk which takes care of all the boilerplate code.

## Installation (TODO)

* Deploy Minio and Etcd Operator
* Deploy CRD
* Deploy Operator (TODO)
* Deploy example application (TODO)

