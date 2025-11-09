FROM golang:1.22-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=go1.24.9

COPY go.mod go.sum ./
RUN go mod download

COPY . .

WORKDIR /app/cmd/public-server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/public-server

FROM alpine:3.20

WORKDIR /app

RUN adduser -D appuser
USER appuser

COPY --from=builder /out/public-server /app/public-server

EXPOSE 82

ENTRYPOINT ["/app/public-server"]
