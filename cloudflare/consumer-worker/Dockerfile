FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY judge-runner/go.mod judge-runner/main.go ./
RUN go build -o /judge-runner .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates python3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /judge-runner /usr/local/bin/judge-runner

WORKDIR /workspace
