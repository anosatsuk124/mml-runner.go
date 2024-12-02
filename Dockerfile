FROM ghcr.io/goreleaser/goreleaser-cross:v1.23.3

RUN apt-get update && apt-get install -y \
  build-essential \
  libssl-dev \
  pkg-config \
  libasound2-dev \
  && rm -rf /var/lib/apt/lists/*

