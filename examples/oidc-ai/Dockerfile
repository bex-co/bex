# The same service for a Docker deploy (runtime: docker). render.yaml deploys
# it on the native Python runtime, which ignores this file.
FROM python:3.13-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY main.py .
CMD ["python", "main.py"]
