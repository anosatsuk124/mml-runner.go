FROM ghcr.io/goreleaser/goreleaser-cross:v1.23.3

RUN dpkg --add-architecture aarch64

RUN apt-get update && apt-get install -y \
  build-essential \
  build-essential:arm64 \
  libssl-dev:arm64 \
  pkg-config \
  libasound2-dev \
  libasound2-dev:arm64 \
  && rm -rf /var/lib/apt/lists/*

