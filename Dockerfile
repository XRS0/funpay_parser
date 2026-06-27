FROM python:3.11

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

ENV DATA_DIR=/app/data
ENV DATABASE_PATH=/app/data/parser.db
ENV DEV_RUNNER=1

RUN mkdir -p /app/data

VOLUME ["/app/data"]
EXPOSE 5000

CMD ["python", "app.py"]
