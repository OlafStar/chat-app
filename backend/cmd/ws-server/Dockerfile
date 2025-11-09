FROM golang:1.22-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=go1.24.9

COPY go.mod go.sum ./
RUN go mod download

COPY . .

WORKDIR /app/cmd/ws-server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ws-server

FROM alpine:3.20

WORKDIR /app

RUN adduser -D appuser
USER appuser

COPY --from=builder /out/ws-server /app/ws-server

EXPOSE 83

ENTRYPOINT ["/app/ws-server"]
