FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY backend/ .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:3.20

RUN adduser -D -g '' appuser
COPY --from=builder /server /server

USER appuser
EXPOSE 8080
ENTRYPOINT ["/server"]
