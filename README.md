# Stack-Cloudscale

## Overview

This `stack-cloudscale` repository is the implementation of a Crossplane infrastructure
[stack](https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md) for
[Cloudscale](https://cloudscale.ch).
The stack that is built from the source code in this repository can be installed into a Crossplane control plane and adds the following new functionality:

* Custom Resource Definitions (CRDs) that model Cloudscale infrastructure and services (currently only S3 supported)
* Controllers to provision these resources on Cloudscale based on the users desired state captured in CRDs they create
* Implementations of Crossplane's [portable resource abstractions](https://crossplane.io/docs/master/running-resources.html), enabling Cloudscale resources to fulfill a user's general need for cloud services

## Getting Started and Documentation

For getting started guides, installation, deployment, and administration, see our [Documentation](https://crossplane.io/docs/latest).

## Release

Build the Docker image and push it:

```console
make -f stack.Makefile docker-build
make -f stack.Makefile docker-push
```
