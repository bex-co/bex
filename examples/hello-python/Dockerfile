FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENV PORT=3000
# Run the app directly (like hello-node's `node main.js`): main.py's __main__
# block binds host 0.0.0.0 on $PORT. `flask run` was wrong here — with no
# FLASK_APP/app.py it exits "Could not locate a Flask application" and crashes
# the container on startup (the deploy reaches build success then update_failed).
CMD ["python", "main.py"]
