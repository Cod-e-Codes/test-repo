# === Build Stage ===
FROM golang:1.24-alpine AS builder
WORKDIR /marchat
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o marchat-server ./cmd/server

# === Runtime Stage ===
FROM alpine:3.21

# Build arguments for user/group ID
ARG USER_ID=1000
ARG GROUP_ID=1000

# Install necessary packages for user management
RUN apk add --no-cache shadow

# Create marchat user with specified UID/GID
RUN groupadd -g ${GROUP_ID} marchat && \
    useradd -u ${USER_ID} -g marchat -s /bin/sh -m marchat

# Create config directory with proper ownership
RUN mkdir -p /marchat/config && \
    chown -R marchat:marchat /marchat

# Switch to marchat user
USER marchat
WORKDIR /marchat

# Copy the binary from builder stage
COPY --from=builder /marchat/marchat-server .

# Copy entrypoint script
COPY --chown=marchat:marchat entrypoint.sh /marchat/entrypoint.sh
RUN chmod +x /marchat/entrypoint.sh

# Expose port 8080
EXPOSE 8080

ENTRYPOINT ["/marchat/entrypoint.sh"]
