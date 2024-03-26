FROM python:3.8

ENV APP_HOME /app
WORKDIR $APP_HOME

COPY requirements.txt ./
RUN pip install --no-cache-dir -r ./requirements.txt

COPY main.py ./

CMD ["python", "main.py"]