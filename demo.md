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

1. Create a curl pod and label for istio injection in the "user1" and "user2" namespace

```
kubectl create ns user1
kubectl create ns user2
```

```
kubectl label namespace user1 istio-injection=enabled --overwrite
kubectl label namespace user2 istio-injection=enabled --overwrite
```

```
kubectl apply -n user1 -f https://raw.githubusercontent.com/istio/istio/master/samples/sleep/sleep.yaml
kubectl apply -n user2 -f https://raw.githubusercontent.com/istio/istio/master/samples/sleep/sleep.yaml
```

2. Create a PeerAuthentication policy to enforce strict mtls

```
kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: user1
spec:
  mtls:
    mode: STRICT
EOF
```

3. Create an AuthorizationPolicy:

```
kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: allow-serving-tests
  namespace: user1
spec:
  action: ALLOW
  rules:
    # 1. mTLS for service from source "user1" namespace to destination service when TargetBurstCapacity=0 without local gateway and activator on the path
    # Source Service from "user1" namespace -> Destination Service in "user1" namespace
    - from:
        - source:
            namespaces: ["user1"]
    # 2. mTLS for service from source "user1" namespace to destination service with activator on the path
    # Source Service from "user1" namespace -> Activator(Knative Serving namespace) -> Destination service in "user1" namespace
    # unfortunately currently we could not lock down the source namespace as Activator does not capture the source namespace when proxying the request, see https://github.com/knative-sandbox/net-istio/issues/554.
    - from:
        - source:
            namespaces: ["knative-serving"]
    # 3. allow metrics and probes from knative serving namespaces
    - from:
        - source:
            namespaces: ["knative-serving"]
      to:
        - operation:
            paths: ["/metrics", "/healthz", "/ready", "/wait-for-drain"]
EOF
```

4. Disable top level VirtualService 

KServe creates an Istio top level VirtuaService to support routing between InferenceService components. To disable the top level virtual service, add the flag "disableIstioVirtualHost": true under the ingress config in inferenceservice configmap.

```
kubectl edit configmap/inferenceservice-config --namespace kserve

ingress : |- {
    "disableIstioVirtualHost": true
}
```

5. Create inference 

```yaml
kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: bert-model
  namespace: user1
  annotations:
    "sidecar.istio.io/inject": "true"
spec:
  predictor:
    containers:
    - image: pj3677/bertmodel:0.0.1
EOF
```

6. Try from user1 and user2 namespaces:

```
kubectl exec -it deployment/sleep -n user1 -c sleep -- curl bert-model-predictor-00001.user1.svc.cluster.local/v1/models/bert-emotion-model
```
