FROM scion-base-isd:1.0

# Copy generated AS11 PKI from base image.
RUN cp /opt/tutorial-pki/AS11/*.key /etc/scion/crypto/as/ && \
    cp /opt/tutorial-pki/AS11/*.pem /etc/scion/crypto/as/ && \
    chown scion:scion /etc/scion/crypto/as/*.key && \
    chmod 600 /etc/scion/crypto/as/*.key

# Copy AS11 topology and configuration files.
COPY topology11.json /etc/scion/topology.json

# If any command in a pipeline fails,
# it should still return a success status.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Generate secret symmetric forwarding keys used by the AS in the data plane
# to verify the MACs in the hop fields of a SCION path (header).
RUN head -c 16 /dev/urandom | base64 > /etc/scion/keys/master0.key && \
    head -c 16 /dev/urandom | base64 > /etc/scion/keys/master1.key

CMD ["/lib/systemd/systemd", "--unit=multi-user.target"]
