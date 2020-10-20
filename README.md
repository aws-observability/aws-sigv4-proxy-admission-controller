# AWS SIGv4 Proxy Admission Controller

The mutation admission controller will inject AWS SIGv4 as a sidecar into container if there is an annotation specified in a container's deployment.yaml file or if a namespace has a specific matching label. A host annotation / namespace label must also be specified so that parameters can be extracted to pass into the container as arguments.

## Getting Started

If you wish to use this repository on Kubernetes, an image exists here: https://gallery.ecr.aws/aws-observability/aws-sigv4-proxy-admission-controller

If you wish to build the image on your own, change the variables in Makefile for your image repo, image name, and tag.

Build and push image
```
make all
```

Build image
```
make build-image
```

Push image
```
make push-image
```

Run tests
```
make test
```

## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
