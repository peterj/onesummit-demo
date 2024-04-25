## Setup

1. Create the cluster:

```bash
kind create cluster
```

2. Use the quick install script to install everything:

```bash
curl -s "https://raw.githubusercontent.com/kserve/kserve/release-0.12/hack/quick_install.sh" | bash
```


3. Install Prometheus, Grafana and Kiali:

```bash
# Install Prometheus and Grafana
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/grafana.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/kiali.yaml
```

## The model

We'll use a BERT model to classify emotions in text. The model is a Hugging Face model that's using the KServe SDKs.

1. Deploy the model to Kubernetes using InferenceService (replace the image name if using a different one):

```yaml
kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: bert-model
  annotations:
    "sidecar.istio.io/inject": "true"
  labels:
    controller-tools.k8s.io: "1.0"
spec:
  predictor:
    containers:
    - image: pj3677/bertmodel:0.0.1
EOF
```

Note: you can either use `pj3677/bertmodel:0.0.1` or `pj3677/bertmodel:2.0.0` images. Alternatively, build the image yourself using the Dockerfile in the `hfmodel` directory.


2. Port-forward the Istio gateway:

```bash
kubectl port-forward svc/istio-ingressgateway -n istio-system 8080:80
```

3. Get the URL where the inference service is exposed on:

```bash
kubectl get inferenceservice
```

```console
NAME         URL                                     READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION          AGE
bert-model   http://bert-model.default.example.com   True           100                              bert-model-predictor-00001   2m39s
```

4. Test the model using `curl`:

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "Istio is the best service mesh"}'
```

5. Show the VirtualService/Gateway/PeerAuthentication resources:

```shell
kubectl get virtualservice,gateway,peerauthentication
```

## Basic Authentication

1. Show the Istio ingress pod:

```shell
kubectl get po -n istio-system --show-labels
```

2. Create an AuthorizationPolicy on the gateway:

```yaml
kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: allow-with-header
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-ingressgateway
  action: ALLOW
  rules:
  - to:
    - operation:
        paths: ["/v1/models/bert-emotion-model:predict"]
    when:
    - key: request.headers[X-Test]
      values: ["istio-is-cool"]
EOF
```

3. Send a request without the header:

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v
```

```console
* Host localhost:8080 was resolved.
* IPv6: ::1
* IPv4: 127.0.0.1
*   Trying [::1]:8080...
* Connected to localhost (::1) port 8080
> POST /v1/models/bert-emotion-model:predict HTTP/1.1
> Host: bert-model.default.example.com
> User-Agent: curl/8.7.1
> Accept: */*
> content-type: application/json
> Content-Length: 51
>
* upload completely sent off: 51 bytes
< HTTP/1.1 403 Forbidden
< content-length: 19
< content-type: text/plain
< date: Tue, 23 Apr 2024 20:59:34 GMT
< server: istio-envoy
< connection: close
<
* Closing connection
RBAC: access denied%
```

4. We can also check the logs in the Istio ingress gateway pod to see the RBAC error (HTTP 403):

```shell
kubectl logs -n istio-system -l app=istio-ingressgateway
```

```console
...
[2024-04-25T20:21:43.725Z] "POST /v1/models/bert-emotion-model:predict HTTP/1.1" 403 - rbac_access_denied_matched_policy[none] - "-" 0 19 0 - "10.244.0.6" "curl/8.4.0" "211ebf7c-1bc2-4008-aa50-9730fa6300dc" "bert-model.default.example.com" "-" outbound|80||knative-local-gateway.istio-system.svc.cluster.local - 127.0.0.1:8080 127.0.0.1:44576 - -
```

5. Send a request with the header: 

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v -H "X-Test: istio-is-cool"
```

```console
* Host localhost:8080 was resolved.
* IPv6: ::1
* IPv4: 127.0.0.1
*   Trying [::1]:8080...
* Connected to localhost (::1) port 8080
> POST /v1/models/bert-emotion-model:predict HTTP/1.1
> Host: bert-model.default.example.com
> User-Agent: curl/8.7.1
> Accept: */*
> content-type: application/json
> X-Test: istio-is-cool
> Content-Length: 51
>
* upload completely sent off: 51 bytes
< HTTP/1.1 200 OK
< content-length: 303
< content-type: application/json
< date: Tue, 23 Apr 2024 21:00:34 GMT
< server: istio-envoy
< x-envoy-upstream-service-time: 4137
<
* Connection #0 to host localhost left intact
{"predictions":[[{"label":"joy","score":0.9889927506446838},{"label":"love","score":0.004175746813416481},{"label":"anger","score":0.003846140578389168},{"label":"sadness","score":0.0012988817179575562},{"label":"fear","score":0.0009755274513736367},{"label":"surprise","score":0.0007109164143912494}]]}%
```

6. Clean up by removing the AuthorizationPolicy:

```shell
kubectl delete authorizationpolicy allow-with-header -n istio-system
```

## Ratelimit

1. Apply ratelimit descriptors to define counts:

```shell
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
          requests_per_unit: 2
        descriptors:
        - key: "x-api-key"
          rate_limit:
            unit: minute
            requests_per_unit: 5
      - key: "path"
        rate_limit:
          unit: minute
          requests_per_unit: 10
EOF
```

2. Apply rate limit service:

```shell
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.21/samples/ratelimit/rate-limit-service.yaml
```

3. Make sure the rate limit service and Redis pods are running:

```shell
kubectl get po
```

4. Apply ratelimit filter to ingress gateway:

```shell
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
      app: istio-ingressgateway
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
```

5. Apply ratelimit actions to bert-model-predictor route:

```shell
kubectl apply -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: filter-ratelimit-svc
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      app: istio-ingressgateway
  configPatches:
    - applyTo: VIRTUAL_HOST
      match:
        context: GATEWAY
        routeConfiguration:
          vhost:
            name: ""
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
                  header_name: "x-api-key"
                  descriptor_key: "x-api-key"
            - actions:
              - request_headers:
                  header_name: ":path"
                  descriptor_key: "path"
EOF
```

6. Send a request to the `v1/models/bert-emotion-model:predict` endpoint, the second request should be ratelimited:

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v
```

7. Add the `api: my-api-key` header to the request to see the rate limiting rate match the 3 requests per minute descriptor:

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v -H "x-api-key: my-api-key"
```

8. Send a request to a different path, the 11th request should be ratelimited: 

```shell
curl -H "Host: bert-model.default.example.com" http://localhost:8080/v1/models/bert-emotion-model  -v
```

```console
{"name":"bert-emotion-model","ready":"True"}
```

The ratelimited response:

```console
❯ curl -H "Host: bert-model.default.example.com" http://localhost:8080/v1/models/bert-emotion-model  -v
* Host localhost:8080 was resolved.
* IPv6: ::1
* IPv4: 127.0.0.1
*   Trying [::1]:8080...
* Connected to localhost (::1) port 8080
> GET /v1/models/bert-emotion-model HTTP/1.1
> Host: bert-model.default.example.com
> User-Agent: curl/8.7.1
> Accept: */*
>
* Request completely sent off
< HTTP/1.1 429 Too Many Requests
< x-envoy-ratelimited: true
< date: Tue, 23 Apr 2024 21:34:49 GMT
< server: istio-envoy
< content-length: 0
<
* Connection #0 to host localhost left intact
```

9. Clean up by removing the EnvoyFilters:

```shell
kubectl delete envoyfilter filter-ratelimit -n istio-system
kubectl delete envoyfilter filter-ratelimit-svc -n istio-system
```

## Wasm plugin

The Wasm plugin in the `stats-plugin` folder is a simple plugin that inspects the responses from the model and increments a counter for the highest scoring emotion.

1. Deploy the plugin to the Istio ingress gateway:

```shell
kubectl apply -f - <<EOF
apiVersion: extensions.istio.io/v1alpha1
kind: WasmPlugin
metadata:
  name: stats-plugin
  namespace: istio-system
spec:
  selector:
    matchLabels:
     app: istio-ingressgateway
  url: oci://docker.io/pj3677/stats-plugin:0.0.1
  imagePullPolicy: Always
EOF
```

2. Send a couple of requests (varying the input sentence):

```shell
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The mood was somber"}' 

curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "Istio is scary"}'

curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The atmosphere was exciting"}'

curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The group was surprised by how much the can do with Istio"}'

curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "Istio is lovely"}'

```

3. You can know check the `/stats/prometheus` endpoint on the ingress gateway to see the metrics. For example:

```shell
kubectl exec -it -n istio-system deploy/istio-ingressgateway -- curl localhost:15000/stats/prometheus | grep sad
```

```console
# TYPE sadness counter
sadness{} 6
```

```shell
kubectl exec -it -n istio-system deploy/istio-ingressgateway -- curl localhost:15000/stats/prometheus | grep joy
```

```console
# TYPE joy counter
joy{} 6
```


```shell
kubectl exec -it -n istio-system deploy/istio-ingressgateway -- curl localhost:15000/stats/prometheus | grep love
```
  
```console
# TYPE love counter
love{} 2
```

## Grafana

1. Let's take a look at these metrics in Grafana:

```shell
istioctl dash grafana
```

2. Import the dashboard using the `grafana-dashboard.json` file.

## Kiali

1. Open kiali:

```shell
istioctl dashboard kiali
```


2. Start sending traffic:

```shell
while true; do curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "Istio is the best service mesh"}'; sleep 1; done
```