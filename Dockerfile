# syntax=docker/dockerfile:1.7

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules across rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=docker
ARG COMMIT=unknown
ARG DATE=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
        -trimpath \
        -ldflags "-s -w \
            -X main.version=${VERSION} \
            -X main.commit=${COMMIT} \
            -X main.date=${DATE}" \
        -o /out/steamgifts-bot \
        ./cmd/steamgifts-bot

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="steamgifts-bot"
LABEL org.opencontainers.image.description="Multi-account giveaway bot for steamgifts.com"
LABEL org.opencontainers.image.source="https://github.com/heinrichb/steamgifts-bot"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app
COPY --from=build /out/steamgifts-bot /app/steamgifts-bot

USER nonroot:nonroot
ENTRYPOINT ["/app/steamgifts-bot"]
CMD ["run", "--config", "/config/config.yaml"]
