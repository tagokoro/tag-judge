FROM rust:1.87-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        gcc \
        git \
        libcap-dev \
        libseccomp-dev \
        libsystemd-dev \
        make \
        pkg-config \
        python3 \
    && useradd --system --create-home isolate \
    && echo "isolate:100000:65536" >> /etc/subuid \
    && echo "isolate:100000:65536" >> /etc/subgid \
    && git clone --depth 1 https://github.com/ioi/isolate.git /tmp/isolate \
    && make -C /tmp/isolate isolate isolate-check-environment \
    && make -C /tmp/isolate install \
    && rm -rf /tmp/isolate \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

CMD ["sleep", "infinity"]
