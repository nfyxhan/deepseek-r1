# pull models
from ollama/ollama:0.5.7 as models
env DS_NAME=deepseek-r1:1.5b
run nohup ollama serve & sleep 5 \
  && ollama pull ${DS_NAME}

# build server
from golang:1.18.10 as golang
workdir /home/workdir

add . .
run go mod tidy && \
    mkdir -p bin && \
    go build -o bin/deepseek main.go && \
    chmod +x start.sh

# build image
from ollama/ollama:0.3.0

COPY --from=models /root/.ollama/models /models

workdir /home/workdir

COPY --from=golang /home/workdir/bin/deepseek /usr/bin
COPY --from=golang /home/workdir/start.sh .

entrypoint ./start.sh