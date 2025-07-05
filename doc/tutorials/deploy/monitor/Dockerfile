FROM alpine:latest

# Install dependencies.
RUN apk add --no-cache tshark

# Default command, override in compose
CMD ["tshark", "-i", "eth0", "-w", "/data/capture.pcap"]