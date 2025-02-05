#!/bin/sh

model_path=/root/.ollama/models

mkdir -p $model_path
rm -rf $model_path
ln -sf /models $model_path

ollama serve &

deepseek serve
