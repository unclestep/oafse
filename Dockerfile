FROM golang:1.26-alpine AS builder

WORKDIR /app
RUN apk add --no-cache make
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make

FROM alpine:latest

RUN apk add --no-cache \
    chromium \
    harfbuzz \
    nss \
    freetype \
    ttf-freefont \
    font-noto \
    ca-certificates \
    dbus

RUN adduser -D -u 10001 crawleruser
WORKDIR /app

COPY --from=builder --chown=crawleruser:crawleruser /app/crawler .

USER 10001:10001
ENTRYPOINT ["/app/crawler"]
