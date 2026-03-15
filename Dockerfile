FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cf-optimizer .

# Final stage
FROM alpine:latest

WORKDIR /root/

# Install curl and ca-certificates
RUN apk --no-cache add curl ca-certificates

# Copy binary from builder
COPY --from=builder /app/cf-optimizer .

# Download nexttrace based on architecture
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        NEXTTRACE_URL="https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_arm64"; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
        NEXTTRACE_URL="https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_amd64"; \
    else \
        echo "Unsupported architecture: $TARGETARCH"; \
        exit 1; \
    fi && \
    curl -L -o nexttrace "$NEXTTRACE_URL" && \
    chmod +x nexttrace

# Expose port
EXPOSE 37377

# Run the application with environment variable support
CMD ["./cf-optimizer"]
