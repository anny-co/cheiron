# anny-co/cheiron

Cheiron is a Kubernetes Operator built from the Operator SDK and written in Go. It maintains imagePullSecrets, both on ServiceAccounts and PodSpecs,
cluster-wide or only namespaced.

## Introduction

### Why build an Operator in the first place?

Becase adding the same imagePullSecret to all Pods or ServiceAccounts added to possibly multiple clusters is tedious for our engineers and we 
want to automate this task as much as possible. Additionally, we wanted to experiment with Kubernetes Operators and this seemed like a reasonable
idea to realize as an operator.

### Why is Cheiron necessary?

Because still, many deployments, Helm Charts, or static manifests use Docker Hub as the container image source. However, due to several licence 
changes, Docker Hub has become heavily rate-limited, which is a particular problem if the cluster is located behind a singular NAT, i.e.,
all kubelets share the same external IP address. In our tests, you exceed this rate-limit already by only deploying a basic set of tools into
a fresh cluster and this hinders progress massively. 

When you cannot simply assign public IP addresses to the cluster nodes, or you want to entirely omit the rate-limit by using credentials to authenticate
the node to Docker Hub (or any other registry for that matter!), you're required to provide ImagePullSecrets for each pod you deploy. This can either
be done directly on the PodSpec or on the ServiceAccount that is attached to the Pod by Kubernetes.

By separation of concern, many Helm charts deploy their own service account for the release. This then requires repeated addition of the imagePullSecret
to the ServiceAccount or the Pod (we think that the former one is more scalable, though). Doing this by hand however is massively time-consuming, even
when using tools such as Kustomize to apply patches to resources automatically.

> **Note that it is also possible to attach the credentials directly to the node running the kubelet. While for high-maintenance clusters or self-provisioned
> nodes this might be feasible, in our scenario, where we have (potentially a lot of) dynamically provisioned nodes without tools such as Ansible in the process,
> we want to perform this inclusion of credentials as little invasive into the node as possible. Hence we do it directly in Kubernetes.**

## Usage

Cheiron follows the latest Operator SDK design, i.e., it is scaffolded from the CLI and automatically build and deployed using the SDK's CLI.

Additionally, we ship static deployment manifests:

```sh
kubectl apply -f config/manifests/
```

After that, the cluster knows the CRDs that Cheiron manages; you can then go ahead and add your image registry credentials in a CR such as

```YAML
apiVersion: cheiron.anny.co/v1alpha1
kind: ImagePullSecretManager
metadata:
  name: my-registry
spec:
  mode: ServiceAccount # or Pod
  secrets:
    - name: docker-hub
      registry: https://index.docker.io/v1
      username: <my-docker-username>
      password: <my-docker-password>
      email: <my-docker-email>
    - name: github-container-registry
      registry: ghcr.io
      username: <my-github-username>
      password: <my-github-access-token>
      email: <my-github-email>
    - existingSecretRef:
        name: gitlab-registry
    - existingSecretRef:
        name: <my-registry>
```

The controller for those resources reconciles the resource spec by either creating a `kubernetes.io/dockerconfigjson` secret or updating an existing one to contain the specified credentials. Then, it collects all the names of the created/updated secrets, and marks either all ServiceAccounts or Pods in its scope (either namespaced or cluster-scoped) reconcilable using annotations:
```YAML
metadata:
  annotations:
    cheiron.anny.co/reconcilable: "true"
```

Note that this is only done on ressources that do not yet have the annotation or those who **don't** have the annotation set to `false`
or **do not** have the specific annotation `cheiron.anny.co/ignore: "true"`.
The controller also adds an annotation that contains all the names of the fresh secrets, s.t. we can construct a list of LocalObjectReferences
on either the `PodSpec.ImagePullSecrets` or `ServiceAccount.ImagePullSecrets`.

> `TODO(docs): add further documentation for the pod_controller.go and serviceaccount_controller.go`