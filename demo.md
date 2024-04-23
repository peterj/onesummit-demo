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

## Basic Authentication 

1. Create an AuthorizationPolicy on the gateway:

```
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

2. Send a request without the header:

``` 
❯ curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v
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

3. Send a request with the header: 

```
❯ curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v -H "X-Test: istio-is-cool"
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

## Ratelimit

1. Apply ratelimit descriptors to define counts:

```
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
            requests_per_unit: 3
      - key: "path"
        rate_limit:
          unit: minute
          requests_per_unit: 10
EOF
```

2. Apply rate limit service: 

```
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.21/samples/ratelimit/rate-limit-service.yaml
```

3. Apply ratelimit filter to ingress gateway:

```
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
```

4. Apply ratelimit actions to bert-model-predictor route:

```
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

5. Send a request to the `v1/models/bert-emotion-model:predict` endpoint, the second request should be ratelimited:

```
curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v
```

6. Add the `api: my-api-key` header to the request to see the rate limiting rate match the 3 requests per minute descriptor:

```
 curl -H "Host: bert-model.default.example.com" -H "content-type: application/json"  http://localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}' -v -H "x-api-key: my-api-key"
```

7. Send a request to a different path, the 11th request should be ratelimited: 

```
> curl -H "Host: bert-model.default.example.com" http://localhost:8080/v1/models/bert-emotion-model  -v

{"name":"bert-emotion-model","ready":"True"}
```

```
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