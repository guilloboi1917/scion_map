#!/bin/bash

set -euo pipefail

mkdir -p /tmp/tutorial-scion-certs-isd04 && cd /tmp/tutorial-scion-certs-isd04
mkdir AS{16..20}

# Create voting and root keys and (self-signed) certificates for core ASes
pushd AS16
scion-pki certificate create --profile=sensitive-voting <(echo '{"isd_as": "18-ffaa:1:1", "common_name": "18-ffaa:1:1 sensitive voting cert"}') sensitive-voting.pem sensitive-voting.key
scion-pki certificate create --profile=regular-voting <(echo '{"isd_as": "18-ffaa:1:1", "common_name": "18-ffaa:1:1 regular voting cert"}') regular-voting.pem regular-voting.key
scion-pki certificate create --profile=cp-root <(echo '{"isd_as": "18-ffaa:1:1", "common_name": "18-ffaa:1:1 cp root cert"}') cp-root.pem cp-root.key
popd

pushd AS17
scion-pki certificate create --profile=cp-root <(echo '{"isd_as": "18-ffaa:1:2", "common_name": "18-ffaa:1:2 cp root cert"}') cp-root.pem cp-root.key
popd

pushd AS18
scion-pki certificate create --profile=sensitive-voting <(echo '{"isd_as": "18-ffaa:1:3", "common_name": "18-ffaa:1:3 sensitive voting cert"}') sensitive-voting.pem sensitive-voting.key
scion-pki certificate create --profile=regular-voting <(echo '{"isd_as": "18-ffaa:1:3", "common_name": "18-ffaa:1:3 regular voting cert"}') regular-voting.pem regular-voting.key
popd

# Create the TRC
mkdir -p tmp
cat <<EOF > trc-B1-S1-pld.tmpl
isd = 18
description = "Demo ISD 18"
serial_version = 1
base_version = 1
voting_quorum = 2

core_ases = ["ffaa:1:1", "ffaa:1:2", "ffaa:1:3"]
authoritative_ases = ["ffaa:1:1", "ffaa:1:2", "ffaa:1:3"]
cert_files = ["AS16/sensitive-voting.pem", "AS16/regular-voting.pem", "AS16/cp-root.pem", "AS17/cp-root.pem", "AS18/sensitive-voting.pem", "AS18/regular-voting.pem"]

[validity]
not_before = $(date +%s)
validity = "365d"
EOF

scion-pki trc payload --out=tmp/ISD18-B1-S1.pld.der --template trc-B1-S1-pld.tmpl
rm trc-B1-S1-pld.tmpl

# Sign and bundle the TRC
scion-pki trc sign tmp/ISD18-B1-S1.pld.der AS16/sensitive-voting.{pem,key} --out tmp/ISD18-B1-S1.AS16-sensitive.trc
scion-pki trc sign tmp/ISD18-B1-S1.pld.der AS16/regular-voting.{pem,key} --out tmp/ISD18-B1-S1.AS16-regular.trc
scion-pki trc sign tmp/ISD18-B1-S1.pld.der AS18/sensitive-voting.{pem,key} --out tmp/ISD18-B1-S1.AS18-sensitive.trc
scion-pki trc sign tmp/ISD18-B1-S1.pld.der AS18/regular-voting.{pem,key} --out tmp/ISD18-B1-S1.AS18-regular.trc

scion-pki trc combine tmp/ISD18-B1-S1.AS{16,18}-{sensitive,regular}.trc --payload tmp/ISD18-B1-S1.pld.der --out ISD18-B1-S1.trc
rm tmp -r

# Create CA key and certificate for issuing ASes
pushd AS16
scion-pki certificate create --profile=cp-ca <(echo '{"isd_as": "18-ffaa:1:1", "common_name": "18-ffaa:1:1 CA cert"}') cp-ca.pem cp-ca.key --ca cp-root.pem --ca-key cp-root.key
popd
pushd AS17
scion-pki certificate create --profile=cp-ca <(echo '{"isd_as": "18-ffaa:1:2", "common_name": "18-ffaa:1:2 CA cert"}') cp-ca.pem cp-ca.key --ca cp-root.pem --ca-key cp-root.key
popd

# Create AS key and certificate chains
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "18-ffaa:1:1", "common_name": "18-ffaa:1:1 AS cert"}') AS16/cp-as.pem AS16/cp-as.key --ca AS16/cp-ca.pem --ca-key AS16/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "18-ffaa:1:2", "common_name": "18-ffaa:1:2 AS cert"}') AS17/cp-as.pem AS17/cp-as.key --ca AS17/cp-ca.pem --ca-key AS17/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "18-ffaa:1:3", "common_name": "18-ffaa:1:3 AS cert"}') AS18/cp-as.pem AS18/cp-as.key --ca AS16/cp-ca.pem --ca-key AS16/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "18-ffaa:1:4", "common_name": "18-ffaa:1:4 AS cert"}') AS19/cp-as.pem AS19/cp-as.key --ca AS16/cp-ca.pem --ca-key AS16/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "18-ffaa:1:5", "common_name": "18-ffaa:1:5 AS cert"}') AS20/cp-as.pem AS20/cp-as.key --ca AS17/cp-ca.pem --ca-key AS17/cp-ca.key --bundle

echo 'copying to shared folder'
cp -r /tmp/tutorial-scion-certs-isd04 /shared/