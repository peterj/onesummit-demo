# Custom Model for kserve

This Python script hosts the `https://huggingface.co/bhadresh-savani/distilbert-base-uncased-emotion` model that detects the emotion in a sentence. The model is hosted using the `kserve` framework.


To test the model out locally, do the following:

1. Create the Python virtual environment and install the dependencies:

```bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

2. Run the Python script:

```bash
python main.py
```

3. Test the model using `curl`.

Sample sentences:

Anger: "Emptiness swallows me whole. I am drowning in its fiery trap"
Joy: "I know tomorrow will be a great day."

```shell
curl -H "content-type: application/json" localhost:8080/v1/models/bert-emotion-model:predict -d '{"input": "The sky is blue and the sun is shining"}'
```

```console
{
  "predictions": [
    [
      {
        "label": "sadness",
        "score": 0.000445224461145699
      },
      {
        "label": "joy",
        "score": 0.9976555109024048
      },
      {
        "label": "love",
        "score": 0.0012775727082043886
      },
      {
        "label": "anger",
        "score": 0.00030572162359021604
      },
      {
        "label": "fear",
        "score": 0.00017205093172378838
      },
      {
        "label": "surprise",
        "score": 0.00014383373491000384
      }
    ]
  ]
}
```


## Issues

- ImportError: cannot import name 'RayServeHandle' from 'ray.serve.handle' --> https://github.com/kserve/kserve/issues/3541. Workaround is to constrain the ray[serve] versions in the requirements.txt 
