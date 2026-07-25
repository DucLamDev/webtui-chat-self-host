# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  for attempt in 1 2 3; do \
    GOPROXY="https://proxy.golang.org|direct" go mod download && break; \
    if [ "$attempt" -eq 3 ]; then exit 1; fi; \
    sleep "$((attempt * 5))"; \
  done

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/webtui-api ./cmd/api
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/webtui-worker ./cmd/worker
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/webtui-migrate ./cmd/migrate

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates postgresql-client tzdata wget \
  && addgroup -S -g 10001 webtui \
  && adduser -S -D -H -u 10001 -G webtui webtui

WORKDIR /app

COPY --from=build /out/webtui-api /usr/local/bin/webtui-api
COPY --from=build /out/webtui-worker /usr/local/bin/webtui-worker
COPY --from=build /out/webtui-migrate /usr/local/bin/webtui-migrate
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY db/migrations ./db/migrations

RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
  && chown -R webtui:webtui /app

ENV APP_ENV=production
ENV DATABASE_MIGRATIONS_PATH=/app/db/migrations

USER webtui

EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["api"]

FROM runtime AS api
CMD ["api"]

FROM runtime AS worker
CMD ["worker"]
