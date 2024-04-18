1. Create the cluster:

```bash
kind create cluster
```

2. Use the quick install script to install everything:

```bash
curl -s "https://raw.githubusercontent.com/kserve/kserve/release-0.12/hack/quick_install.sh" | bash
```

3. Build & push the docker image from the `hfmodel` folder:

```bash
docker build -t pj3677/bertmodel:0.0.1 .
```

4. Deploy the model to Kubernetes using InferenceService (replace the image name if using a different one):

```yaml
kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: bert-model
spec:
  predictor:
    containers:
    - image: pj3677/bertmodel:0.0.1
EOF
```


## Testing the inference service

1. Port-forward the Istio gateway:

```bash
kubectl port-forward svc/istio-ingressgateway -n istio-system 8080:80
```

2. Get the URL where the inference service is exposed on:

```bash
kubectl get inferenceservice
```

```console
NAME         URL                                     READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION          AGE
bert-model   http://bert-model.default.example.com   True           100                              bert-model-predictor-00001   2m39s
```

3. Test the model using `curl`:

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}'
```

## Ratelimit 

### Global ratelimiting 

kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ratelimit-config
data:
  config.yaml: |
    domain: ratelimit
    descriptors:
      - key: "path"
        value: "/v1/models/bert-emotion-model:predict"
        rate_limit:
          unit: minute
          requests_per_unit: 1
        descriptors:
        - key: "api"
          rate_limit:
            unit: minute
            requests_per_unit: 3
      - key: "path"
        rate_limit:
          unit: minute
          requests_per_unit: 100
EOF

kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.21/samples/ratelimit/rate-limit-service.yaml

kubectl apply -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: filter-ratelimit
  namespace: istio-system
spec:
  workloadSelector:
    # select by label in the same namespace
    labels:
      istio: ingressgateway
  configPatches:
    # The Envoy config you want to modify
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.router"
      patch:
        operation: INSERT_BEFORE
        # Adds the Envoy Rate Limit Filter in HTTP filter chain.
        value:
          name: envoy.filters.http.ratelimit
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit
            # domain can be anything! Match it to the ratelimter service config
            domain: ratelimit
            failure_mode_deny: true
            timeout: 10s
            rate_limit_service:
              grpc_service:
                envoy_grpc:
                  cluster_name: outbound|8081||ratelimit.default.svc.cluster.local
                  authority: ratelimit.default.svc.cluster.local
              transport_api_version: V3
EOF

kubectl apply -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: filter-ratelimit-svc
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
    - applyTo: VIRTUAL_HOST
      match:
        context: GATEWAY
        routeConfiguration:
          vhost:
            name: "bert-model-predictor.default.svc.cluster.local:8081"
            route:
              action: ANY
      patch:
        operation: MERGE
        # Applies the rate limit rules.
        value:
          rate_limits:
            - actions:
              - request_headers:
                  header_name: ":path"
                  descriptor_key: "path"
              - request_headers:
                  header_name: "api"
                  descriptor_key: "api"
EOF

Send a request to the `v1/models/bert-emotion-model:predict` endpoint, the second request should be ratelimited:

```
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v
```

Add the `api: my-api-key` header to the request to see the rate limiting rate match the 3 requests per minute descriptor:

```
 curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v -H "api: my-api-key"
```


