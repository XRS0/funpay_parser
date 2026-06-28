FROM node:25-bookworm AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/${SERVICE}

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/service /app/service
COPY --from=frontend /src/frontend/dist /app/frontend/dist
ENV DATA_DIR=/app/data
ENV DATABASE_PATH=/app/data/parser.db
ENV PORT=5000
RUN mkdir -p /app/data
VOLUME ["/app/data"]
EXPOSE 5000 9090 9091
CMD ["/app/service"]
