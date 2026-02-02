FROM golang:1.24

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      python3-dev \
      libzmq3-dev \
      pkg-config && \
    rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    TOKENIZERS_VERSION=1.22.1; \
    curl -fsSL \
      https://github.com/daulet/tokenizers/releases/download/v${TOKENIZERS_VERSION}/libtokenizers.linux-arm64.tar.gz \
      -o /tmp/libtokenizers.linux-arm64.tar.gz; \
    mkdir -p /tmp/libtokenizers; \
    tar -xzf /tmp/libtokenizers.linux-arm64.tar.gz -C /tmp/libtokenizers; \
    find /tmp/libtokenizers -type f -name 'libtokenizers.*' -exec cp {} /usr/local/lib/ \;; \
    ldconfig; \
    ls -la /usr/local/lib | grep -i tokenizers; \
    test -f /usr/local/lib/libtokenizers.a || test -f /usr/local/lib/libtokenizers.so
