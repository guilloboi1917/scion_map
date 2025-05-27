# syntax = docker/dockerfile:1.3
FROM --platform=$BUILDPLATFORM debian:12-slim

ARG TARGETARCH
ARG SCION_VER=0.12.0
ARG RELEASE="https://github.com/scionproto/scion/releases/download/v${SCION_VER}/scion_${SCION_VER}_deb_${TARGETARCH}.tar.gz"

# Tell systemd it's running inside Docker. Without this, systemd may emit warnings
# and fail to boot certain units (especially networking-related).
ENV container=docker

# Force debconf (called by apt-get) to be non-interactive.
# This prevents interactive prompts during package installation.
ENV DEBIAN_FRONTEND=noninteractive
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# Install dependencies.
RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd libcap2-bin \
    curl wget tar ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install SCION packages. Eventually, also remove the apt cache files
# stored under /var/lib/apt/lists/ to reduce the image size.
RUN mkdir -p /tmp/scion/ && \
    wget --progress=dot:giga "$RELEASE" -O /tmp/scion/pkgs.tar.gz && \
    tar -xzf /tmp/scion/pkgs.tar.gz -C /tmp/scion/ && \
    ls -l /tmp/scion/ && \
    apt-get install -y --no-install-recommends $(find /tmp/scion -name '*.deb') && \
    rm -rf /tmp/scion /var/lib/apt/lists/*

# Copy systemd unit files
COPY systemd/ /etc/systemd/system/

# Enable required SCION services
RUN systemctl enable scion-daemon.service \
    scion-dispatcher.service \
    scion-control@cs.service \
    scion-router@br.service

# Create SCION configuration directories.
RUN mkdir -p \
    /etc/scion/crypto/as \
    /etc/scion/certs \
    /etc/scion/keys \
    /etc/scion/config

# Create shared folder
RUN mkdir -p /shared

COPY br.toml /etc/scion/br.toml
COPY cs.toml /etc/scion/cs.toml

WORKDIR /pki

# Setup PKI.
COPY pki-generation-isd01.bash /pki/
COPY pki-generation-isd02.bash /pki/
COPY pki-generation-isd03.bash /pki/
RUN chmod +x pki-generation-isd01.bash && ./pki-generation-isd01.bash
RUN chmod +x pki-generation-isd02.bash && ./pki-generation-isd02.bash
RUN chmod +x pki-generation-isd03.bash && ./pki-generation-isd03.bash

# Persist the generated ISD TRC file.
RUN ls -l /tmp/tutorial-scion-certs-isd01 && mv /tmp/tutorial-scion-certs-isd01/ISD15-B1-S1.trc /etc/scion/certs/
RUN ls -l /tmp/tutorial-scion-certs-isd02 && mv /tmp/tutorial-scion-certs-isd02/ISD16-B1-S1.trc /etc/scion/certs/
RUN ls -l /tmp/tutorial-scion-certs-isd03 && mv /tmp/tutorial-scion-certs-isd03/ISD17-B1-S1.trc /etc/scion/certs/

# Persist the generated files.
RUN mkdir -p /opt/tutorial-pki && \
    cp -r /tmp/tutorial-scion-certs-isd01/* /opt/tutorial-pki/ && \
    cp -r /tmp/tutorial-scion-certs-isd02/* /opt/tutorial-pki/ && \
    cp -r /tmp/tutorial-scion-certs-isd03/* /opt/tutorial-pki/