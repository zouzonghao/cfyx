FROM alpine:latest

WORKDIR /root/

RUN apk --no-cache add curl ca-certificates unzip

ARG TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        NEXTTRACE_URL="https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_arm64"; \
        XRAY_ASSET="Xray-linux-arm64-v8a.zip"; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
        NEXTTRACE_URL="https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_amd64"; \
        XRAY_ASSET="Xray-linux-64.zip"; \
    else \
        echo "Unsupported architecture: $TARGETARCH"; \
        exit 1; \
    fi && \
    curl -L -o nexttrace "$NEXTTRACE_URL" && \
    chmod +x nexttrace && \
    curl -L -o xray.zip "https://github.com/XTLS/Xray-core/releases/latest/download/$XRAY_ASSET" && \
    unzip -o xray.zip xray -d ./ && \
    chmod +x xray && \
    rm xray.zip

ARG TARGETARCH
COPY binaries/cf-optimizer-linux-${TARGETARCH} cf-optimizer

RUN chmod +x cf-optimizer

EXPOSE 37377

CMD ["./cf-optimizer"]
