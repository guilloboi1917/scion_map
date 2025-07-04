FROM alpine:latest

# Install dependencies.
RUN apt-get update && apt-get install -y --no-install-recommends wireshark && \
    rm -rf /tmp/scion /var/lib/apt/lists/*

# Default command (override in compose)
CMD ["tshark", "--help"]  