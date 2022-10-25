# What is `kcp`?

`kcp` is a prototype of a multi-tenant Kubernetes control plane for workloads on many clusters. `kcp` can be used to manage Kubernetes-like applications across one or more clusters and integrate them with cloud services. 

It provides a generic CRD apiserver that is divided into multiple logical clusters (in which each of the logical clusters are fully isolated) that enable multi-tenancy of cluster-scoped resources such as CRDs and Namespaces. 

See the [`kcp` docs](https://github.com/Kuadrant/kcp) for further explanation and to learn more about the terminology, refer to the [docs](https://github.com/kcp-dev/kcp/blob/e798d372575e694db66bff61bd46700b413e1b9e/docs/content/en/main/concepts/concepts.md).


# What is GLBC?

The Global Load Balancer Controller (GLBC) solves multi-cluster ingress use cases while leveraging `kcp` to provide transparent multi-cluster deployments.

Currently, the GLBC is deployed in a Kubernetes cluster, referred to as the GLBC control cluster, outside the `kcp` control plane. The GLBC dependencies, such as `cert-manager` (and eventually `external-dns`), are deployed alongside the GLBC in that control cluster. These components coordinate through a shared state that persists in the control cluster data plane.

The following benefits/use cases envisioned for GLBC include:

- Provide a single host that can be used to access workloads and bring traffic to the correct physical clusters. 
   - The GLBC manages the DNS for this host and provides a valid TLS certificate. If a workload moves or expands contracts across clusters, GLBC ensures that the DNS for this host is correct and traffic will continue to reach the workload.
- Leverage the data durability guarantees provided by hosted `kcp` environments
- Compute commoditization and workload movement


# Architecture

See the [architecture diagram](/docs/architecture.md) for more information. 

# Terms to Know
KCP : The upstream open source project that provides the technology for the control plane service and workload service.

Global Load Balancer Controller (GLBC): The upstream open source project that provides the global load balancing capabilities to enable multi cluster ingress
