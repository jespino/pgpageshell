FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

# PostgreSQL client (useful for generating test data files)
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends postgresql-client \
    && rm -rf /var/lib/apt/lists/*
