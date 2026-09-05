FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk --no-cache add ca-certificates tzdata bash curl procps

WORKDIR /

COPY go-overlay /go-overlay

RUN chmod 0755 /go-overlay

COPY services.toml /services.toml

ENTRYPOINT ["/go-overlay"]
