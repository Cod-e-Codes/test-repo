# === Build Stage ===
FROM golang:1.24-alpine AS builder
WORKDIR /marchat
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o marchat-server ./cmd/server

# === Runtime Stage ===
FROM alpine:latest
RUN adduser -D marchat
USER marchat
WORKDIR /marchat
COPY --from=builder /marchat/marchat-server .
EXPOSE 8080
ENTRYPOINT ["./marchat-server"]
