# AWS SIGv4 Proxy Admission Controller

The mutation admission controller will inject the [AWS SIGv4 Proxy](https://github.com/awslabs/aws-sigv4-proxy) as a sidecar into a pod if there are annotations specified in a container's deployment.yaml file or specific namespace labels.

## Getting Started

A helm chart exists to deploy all the resources needed to use the admission controller here: https://github.com/aws/eks-charts/tree/master/stable/aws-sigv4-proxy-admission-controller/.

### Installing the Controller via Helm Chart

Add the EKS repository to Helm:

```bash
helm repo add eks https://aws.github.io/eks-charts
```

Install the AWS SIGv4 Admission Controller chart with default configuration:

```bash
helm install aws-sigv4-proxy-admission-controller eks/aws-sigv4-proxy-admission-controller --namespace <namespace>
```

### Uninstalling the Helm Chart

To uninstall/delete the `aws-sigv4-proxy-admission-controller` release:

```bash
helm uninstall aws-sigv4-proxy-admission-controller --namespace <namespace>
```

### Doing It Yourself

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

You can override the admission controller image and other parameters in the [admission controller helm chart](https://github.com/aws/eks-charts/tree/master/stable/aws-sigv4-proxy-admission-controller).

## Usage

### Configuration

For each row in the chart below, you only need either the annotation or namespace label.

| Annotation | Namespace Label | Required
| - | - | -
| `sidecar.aws.signing-proxy/inject: true` | `sidecar-inject=true` | ✔
| `sidecar.aws.signing-proxy/host: <AWS_SIGV4_PROXY_HOST>` | `sidecar-host=<AWS_SIGV4_PROXY_HOST>` | ✔
| `sidecar.aws.signing-proxy/name: <AWS_SIGV4_PROXY_NAME>` | `sidecar-host=<AWS_SIGV4_PROXY_NAME>` |
| `sidecar.aws.signing-proxy/region: <AWS_SIGV4_PROXY_REGION>` | `sidecar-host=<AWS_SIGV4_PROXY_REGION>` |
| `sidecar.aws.signing-proxy/role-arn: <AWS_SIGV4_PROXY_ROLE_ARN>` | `sidecar-role-arn=<AWS_SIGV4_PROXY_ROLE_ARN>` |

For more information on the above annotations / namespace labels, please refer to the documentation in the [AWS SIGv4 Proxy](https://github.com/awslabs/aws-sigv4-proxy) repository.

#### Example Deployment
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sleep
  namespace: sidecar
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sleep
  template:
    metadata:
      annotations:
        sidecar.aws.signing-proxy/inject: "true"
        sidecar.aws.signing-proxy/host: "aps.us-west-2.amazonaws.com"
        sidecar.aws.signing-proxy/name: "aps"
        sidecar.aws.signing-proxy/region: "us-west-2"
        sidecar.aws.signing-proxy/role-arn: "arn:aws:iam::123456789:role/assume-role"
      labels:
        app: sleep
    spec:
      containers:
      - name: sleep
        image: tutum/curl
        command: ["/bin/sleep","infinity"]
        imagePullPolicy: IfNotPresent
```

To see the AWS SIGv4 Proxy installed as a sidecar in this deployment: save the above lines as a yaml file, make sure the admission controller helm chart is installed in your Kubernetes cluster, and run the following:

```bash
kubectl create namespace sidecar
kubectl create -f test-deploy.yaml
kubectl get pod -n sidecar
```

2 pods should be visible within the sleep pod.

## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
