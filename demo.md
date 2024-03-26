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