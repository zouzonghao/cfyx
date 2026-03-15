FROM alpine:latest

WORKDIR /root/

RUN apk --no-cache add curl ca-certificates

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

ARG TARGETARCH
COPY binaries/cf-optimizer-linux-${TARGETARCH} cf-optimizer

RUN chmod +x cf-optimizer

EXPOSE 37377

CMD ["./cf-optimizer"]
