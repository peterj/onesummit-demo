from typing import Dict 
import kserve
from transformers import pipeline
from kserve import ModelServer
import logging

# https://huggingface.co/bhadresh-savani/distilbert-base-uncased-emotion
MODEL_NAME="bhadresh-savani/distilbert-base-uncased-emotion"

class KServeBERTEmotionModel(kserve.Model):

    def __init__(self, name: str):
        super().__init__(name)
        KSERVE_LOGGER_NAME = 'kserve'
        self.logger = logging.getLogger(KSERVE_LOGGER_NAME)
        self.name = name
        self.ready = False

    def load(self):
        self.classifier = pipeline("text-classification",model=MODEL_NAME, top_k=None)
        self.ready = True

    def predict(self, request: Dict, headers: Dict) -> Dict:
        self.logger.info(f"Request: {request}")
        input_sentence = request["input"]
        self.logger.info(f"input: {input_sentence}")

        prediction = self.classifier(input_sentence, )
        self.logger.info(f"results:-- {prediction}")

        return {"predictions": prediction}

if __name__ == "__main__":

  model = KServeBERTEmotionModel("bert-emotion-model")
  model.load()

  model_server = ModelServer(http_port=8080, workers=1)
  model_server.start([model])