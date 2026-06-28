# Funpay Parser

Универсальный веб-парсер маркетплейса **Funpay** на **Go** с параллельным парсингом страниц/описаний и параллельной классификацией лотов через LLM (Fireworks или OpenRouter). Позволяет искать товары в любой категории, фильтровать и классифицировать объявления, сохранять результаты и настраивать автоматический запуск.

## Возможности

- Параллельный парсинг любой категории Funpay или поискового запроса через goroutines/worker pool.
- Параллельная классификация лотов LLM по пользовательским критериям (тип аккаунта, подписка, личный/общий и т.д.).
- Фильтрация по длительности/периоду, указанному в запросе.
- Профили поиска с сохранением настроек для разных категорий и товаров.
- Сохранённые результаты и история запусков.
- Автоматический запуск по расписанию.
- Настройка LLM провайдера и API ключа через веб-интерфейс.
- Остановка запущенной обработки в любой момент.

## Требования

- Docker и Docker Compose
- Go 1.24+ для локального запуска без Docker
- Fireworks или OpenRouter API ключ
- Прокси для запросов к Funpay (по умолчанию SOCKS5, формат `host:port@user:pass`)

## Запуск

1. Создай файл `.env` в корне проекта:

```env
# Fireworks (по умолчанию)
FIREWORKS_API_KEY=your_fireworks_api_key

# Или OpenRouter
# LLM_PROVIDER=openrouter
# OPENROUTER_API_KEY=your_openrouter_api_key

PROXY=host:port@user:pass
```

2. Запусти контейнер:

```bash
docker compose up --build -d
```

3. Открой приложение в браузере:

```
http://localhost:5000
```

Данные (SQLite, настройки, состояние запуска) сохраняются в локальной папке `./data` через Docker volume.

## Страницы приложения

- `/` — главная страница парсера, запуск по профилю или вручную.
- `/saved` — история сохранённых результатов.
- `/scheduler` — управление расписанием автоматических запусков.
- `/settings` — настройка LLM провайдера и API ключа.

## Пример использования

По умолчанию интерфейс настроен на категорию **ChatGPT Plus** (Funpay category ID `1355`), но ты можешь легко переключиться на любую другую категорию, указав нужный `category_id` и поисковый запрос в форме или в профиле.

Встроенные пресеты запросов: `ChatGPT Plus`, `ChatGPT Team`, `Midjourney`, `Netflix` — их можно заменить или расширить в коде интерфейса.

## Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|------------|----------|----------------------|
| `LLM_PROVIDER` | Провайдер LLM: `fireworks` или `openrouter` | `fireworks` |
| `FIREWORKS_API_KEY` | Ключ для Fireworks API | — |
| `FIREWORKS_MODEL` | Модель Fireworks | `accounts/fireworks/models/llama-v3p1-70b-instruct` |
| `OPENROUTER_API_KEY` | Ключ для OpenRouter API | — |
| `OPENROUTER_MODEL` | Модель OpenRouter | `openai/gpt-4o-mini` |
| `PROXY` | Прокси для Funpay в формате `host:port@user:pass` | — |
| `FUNPAY_BASE_URL` | Базовый URL Funpay | `https://funpay.com` |
| `FUNPAY_LANG` | Язык интерфейса Funpay | `en` |
| `MAX_PAGES` | Максимальное количество страниц для парсинга | `3` |
| `DATABASE_PATH` | Путь к SQLite внутри контейнера | `/app/data/parser.db` |
| `DATA_DIR` | Директория для настроек и состояния | `/app/data` |

## Структура проекта

```
.
├── main.go                         # Go HTTP server entrypoint
├── internal/
│   ├── config/                     # ENV + persisted settings
│   ├── duration/                   # Извлечение и фильтрация по длительности
│   ├── llm/                        # LLM client + parallel worker pool
│   ├── models/                     # Модели данных лотов
│   ├── runner/                     # Оркестрация пайплайна парсинга
│   ├── scraper/                    # FunPay scraper + parallel page/deep fetch
│   ├── store/                      # SQLite storage layer
│   └── web/                        # API, UI routes, run state, scheduler loop
├── frontend/                       # React/Vite SPA
│   ├── src/                        # React-компоненты, страницы и стили
│   └── public/                     # Статичные frontend-ассеты
├── go.mod / go.sum                 # Go-зависимости
├── Dockerfile                      # Multi-stage Go сборка
└── docker-compose.yml              # Docker Compose сервисы
```

Локальный запуск без Docker:

```bash
npm install --prefix frontend
npm run build --prefix frontend
go run ./
```

## Примечания

- Все запросы к Funpay всегда выполняются через прокси.
- LLM провайдер и API ключ можно задать в `.env` или в веб-интерфейсе. Настройки из UI сохраняются в `data/settings.json` и имеют приоритет над переменными окружения.
- Результаты сохраняются в `results.json` и в базу данных.
- При перезагрузке страницы восстанавливается только **текущий** запуск; завершённые запуски не сохраняются в интерфейсе.
- LLM-классификатор настроен на определение типа аккаунта/подписки, но может быть адаптирован под любую категорию товаров.


## Микросервисная архитектура

Проект теперь разделён на сервисы: API/Web, Parser Service, LLM Service и Telegram Service. Синхронные вызовы идут по gRPC, событие найденного предложения публикуется в Kafka-compatible брокер Redpanda и потребляется Telegram Service. Подробности см. в [`docs_microservices.md`](docs_microservices.md).
