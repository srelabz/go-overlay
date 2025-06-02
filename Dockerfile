FROM golang:1.23.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o go-supervisor main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata bash curl procps

WORKDIR /

COPY --from=builder /app/go-supervisor /go-supervisor

RUN chmod +x /go-supervisor

COPY services.toml /services.toml

ENTRYPOINT ["/go-supervisor"]

