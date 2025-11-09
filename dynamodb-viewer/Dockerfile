# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o viewer

FROM alpine:3.20
WORKDIR /app
RUN adduser -D viewer && chown viewer /app
USER viewer
COPY --from=build /app/viewer ./viewer
EXPOSE 4100
ENV VIEWER_BIND_ADDR=:4100 \
    AWS_REGION=eu-central-1
CMD ["./viewer"]
