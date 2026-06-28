FROM golang:1.26-alpine AS builder
ARG BINARY=ddns-watchdog-client
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o /out/app ./cmd/${BINARY}/

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 ddns
COPY --from=builder /out/app /usr/local/bin/app
LABEL org.opencontainers.image.source="https://github.com/y1jiong/ddns-watchdog"
LABEL org.opencontainers.image.description="Dynamic DNS update tool"
LABEL org.opencontainers.image.licenses="Apache-2.0"
USER ddns
ENTRYPOINT ["/usr/local/bin/app"]
