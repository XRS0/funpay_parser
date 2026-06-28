# Microservice architecture

Проект разделён на четыре Go-сервиса и Kafka-compatible брокер.

## Services

| Service | Binary | Responsibility | Sync comms | Async comms |
|---|---|---|---|---|
| API/Web | `cmd/api` | UI, settings, profiles, schedules, run state | calls ParserService over gRPC when `PARSER_SERVICE_ADDR` is set | publishes `deals.found` to Kafka when a cheapest deal is found |
| Parser Service | `cmd/parser-service` | FunPay scraping, duration/candidate selection, parser orchestration | calls LLMService over gRPC when `LLM_SERVICE_ADDR` is set | none |
| LLM Service | `cmd/llm-service` | Fireworks/OpenRouter classification worker pool | exposes `funpay.LLMService/ClassifyMany` over gRPC | none |
| Telegram Service | `cmd/telegram-service` | sends Telegram notifications | none | consumes `deals.found` from Kafka |
| Kafka | Redpanda | Kafka protocol broker | n/a | topic `deals.found` |

## Communication choices

- **gRPC** is used for synchronous request/response flows:
  - API → Parser Service (`RunParser`)
  - Parser Service → LLM Service (`ClassifyMany`)
- **Kafka** is used for fire-and-forget notification events:
  - API publishes `deals.found`
  - Telegram Service consumes and sends a message

The gRPC layer uses a registered JSON codec to avoid generated protobuf code while still using real gRPC transport and service boundaries.

## Important environment variables

### API

```env
PARSER_SERVICE_ADDR=parser-service:9090
KAFKA_BROKERS=kafka:9092
KAFKA_DEAL_TOPIC=deals.found
DATABASE_PATH=/app/data/parser.db
DATA_DIR=/app/data
```

### Parser Service

```env
LLM_SERVICE_ADDR=llm-service:9091
PROXY=host:port@user:pass       # FunPay-only proxy
FUNPAY_BASE_URL=https://funpay.com
FUNPAY_LANG=en
MAX_PAGES=3
```

### LLM Service

```env
LLM_PROVIDER=fireworks
FIREWORKS_API_KEY=...
FIREWORKS_MODEL=...
OPENROUTER_API_KEY=...
OPENROUTER_MODEL=...
```

### Telegram Service

```env
KAFKA_BROKERS=kafka:9092
KAFKA_DEAL_TOPIC=deals.found
TELEGRAM_BOT_TOKEN=...
TELEGRAM_CHAT_ID=...
TELEGRAM_PROXY=socks5://127.0.0.1:10808  # Telegram-only VPN/proxy
```

`PROXY` and `TELEGRAM_PROXY` are intentionally separate: FunPay traffic uses `PROXY`, Telegram API uses `TELEGRAM_PROXY`.

## Local Docker run

```bash
docker compose up --build
```

Main UI:

```text
http://localhost:5000
```

Kafka/Redpanda listeners:

- services inside Docker use `kafka:9092`;
- host tools/tests can use `localhost:19092`.

## Fallback mode

The API binary still supports local fallback for development:

- if `PARSER_SERVICE_ADDR` is empty, API uses the in-process parser runner;
- if `KAFKA_BROKERS` is empty, API sends Telegram directly instead of publishing a Kafka event.

This keeps `go run ./` usable during development while Docker Compose runs the full microservice topology.
