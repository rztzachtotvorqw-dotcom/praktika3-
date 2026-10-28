# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /build

COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify

COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/app /app/app
COPY frontend/ /app/static/

USER nonroot:nonroot

ENV HTTP_PORT=8080 \
    STATIC_DIR=/app/static

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app/app", "-healthcheck"]

ENTRYPOINT ["/app/app"]
