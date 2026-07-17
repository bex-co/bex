FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENV PORT=3000
CMD ["sh", "-c", "flask run --host=0.0.0.0 --port=${PORT}"]
