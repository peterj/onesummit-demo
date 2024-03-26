# OneSummit conference talk

When operating Machine Learning workflows there are many moving pieces that are required to serve ML models. When models change, it is challenging to generate the logic for shifting to a new model, while ensuring that security and resiliency is maintained. If using Kubeflow and KServe, things that need to be considered such as security, routing, and observability, are abstracted away. Under the hood, Istio is doing all the heavy lifting, handling the configuration needed for everything from security with AuthN and AuthZ policies to canary deployments. We will cover topics including:

- How Istio can help with exposing existing and canary releasing newer models.
- The minimum viable Istio configuration to allow for secure ML operations.
- How KServe’s CRs are translated into Istio resources
- Configuration for traffic mirroring, fault injection, failover, rate-limit, external authentication, and BYO wasm plugins with Istio alongside the generated Kubeflow and KServe configurations