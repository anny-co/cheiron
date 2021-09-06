# anny-co/cheiron

> NOTE: Cheiron is currently in very early stages of development and and far
> from anything usable. Feel free to contribute if you want to, an(n)y
> contributions are welcome!

Cheiron is a Kubernetes Operator built from the Operator SDK and written in Go.
It maintains imagePullSecrets, both on ServiceAccounts and PodSpecs,
cluster-wide or only namespaced.

## Introduction

### Why build an Operator in the first place?

Becase adding the same imagePullSecret to all Pods or ServiceAccounts added to
possibly multiple clusters is tedious for our engineers and we want to automate
this task as much as possible. Additionally, we wanted to experiment with
Kubernetes Operators and this seemed like a reasonable idea to realize as an
operator.

### Why is Cheiron necessary?

Because still, many deployments, Helm Charts, or static manifests use Docker Hub
as the container image source. However, due to several licence changes, Docker
Hub has become heavily rate-limited, which is a particular problem if the
cluster is located behind a singular NAT, i.e., all kubelets share the same
external IP address. In our tests, you exceed this rate-limit already by only
deploying a basic set of tools into a fresh cluster and this hinders progress
massively. 

When you cannot simply assign public IP addresses to the cluster nodes, or you
want to entirely omit the rate-limit by using credentials to authenticate the
node to Docker Hub (or any other registry for that matter!), you're required to
provide ImagePullSecrets for each pod you deploy. This can either be done
directly on the PodSpec or on the ServiceAccount that is attached to the Pod by
Kubernetes.

By separation of concern, many Helm charts deploy their own service account for
the release. This then requires repeated addition of the imagePullSecret to the
ServiceAccount or the Pod (we think that the former one is more scalable,
though). Doing this by hand however is massively time-consuming, even when using
tools such as Kustomize to apply patches to resources automatically.

> **Note that it is also possible to attach the credentials directly to the node
> running the kubelet. While for high-maintenance clusters or self-provisioned
> nodes this might be feasible, in our scenario, where we have (potentially a
> lot of) dynamically provisioned nodes without tools such as Ansible in the
> process, we want to perform this inclusion of credentials as little invasive
> into the node as possible. Hence we do it directly in Kubernetes.**

### But is this actually good practice?

The quick disclaimer: **Probably not!**. When using the namespaced version,
things are slightly less anti-pattern prone. The operator runs on a opt-out
basis, i.e., when not explicitly flagged, each Pod or ServiceAccount will get
augmented with **all** secrets that the CR creator specified. Kubernetes handles
the secrets, but inlining the secret into the CR will store the credentials in
plain-text in etcd *when not encrypting all data in etcd at rest*.

When using a cluster-scoped version, the operator will create secrets for all
specified registry credentials **in ALL namespaces**, i.e., will inject likely
private data into all namespaces. This is definitly not feasible in clusters
with multiple tenants, and should only be used in scenarios where the entire
cluster is a trusted environment.

In our scenario, this is the case, hence we wrote this operator. We needed to
automate as much of the tedious work of adding explicit pull secrets to 
public registries as possible. You should probably also make a careful evaluation
before going ahead and adding the operator to your cluster.

## Usage

Cheiron follows the latest Operator SDK design, i.e., it is scaffolded from the
CLI and automatically build and deployed using the SDK's CLI.

Additionally, we ship static deployment manifests:

```sh
kubectl apply -f config/manifests/
```

After that, the cluster knows the CRDs that Cheiron manages; you can then go
ahead and add your image registry credentials in a CR such as

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

The controller for those resources reconciles the resource spec by either
creating a `kubernetes.io/dockerconfigjson` secret or updating an existing one
to contain the specified credentials. Then, it collects all the names of the
created/updated secrets, and marks either all ServiceAccounts or Pods in its
scope (either namespaced or cluster-scoped) reconcilable using annotations:
```YAML
metadata:
  annotations:
    cheiron.anny.co/reconcilable: "true"
    cheiron.anny.co/reconcile-with: "secret-name-a,secret-name-b,..."
```

Note that this is only done on ressources that do not yet have the annotation or
those who **don't** have the annotation set to `false` or **do not** have the
specific annotation `cheiron.anny.co/ignore: "true"`. The controller also adds
an annotation that contains all the names of the fresh secrets, s.t. we can
construct a list of LocalObjectReferences on either the
`PodSpec.ImagePullSecrets` or `ServiceAccount.ImagePullSecrets`.

The *PodController* and *ServiceAccountController* perform similar operations on
their respective resources: As the first controller adds the annotations from
above, their (the pod controller's and service account controller's)
reconciliation loop picks up the annotated resource and adds the inlined image
pull secrets to the specs. They finish their operations by annotating their
ressource with `cheiron.anny.co/reconciled: "true"`. This allows the controllers
to filter what resources they already worked.