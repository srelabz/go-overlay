FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata bash curl procps

WORKDIR /

COPY go-overlay /go-overlay

RUN chmod +x /go-overlay

COPY services.toml /services.toml

ENTRYPOINT ["/go-overlay"]

