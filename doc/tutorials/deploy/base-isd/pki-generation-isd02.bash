#!/bin/bash

set -euo pipefail

mkdir -p /tmp/tutorial-scion-certs-isd02 && cd /tmp/tutorial-scion-certs-isd02
mkdir AS{6..10}

# Create voting and root keys and (self-signed) certificates for core ASes
pushd AS6
scion-pki certificate create --profile=sensitive-voting <(echo '{"isd_as": "16-ffaa:1:1", "common_name": "16-ffaa:1:1 sensitive voting cert"}') sensitive-voting.pem sensitive-voting.key
scion-pki certificate create --profile=regular-voting <(echo '{"isd_as": "16-ffaa:1:1", "common_name": "16-ffaa:1:1 regular voting cert"}') regular-voting.pem regular-voting.key
scion-pki certificate create --profile=cp-root <(echo '{"isd_as": "16-ffaa:1:1", "common_name": "16-ffaa:1:1 cp root cert"}') cp-root.pem cp-root.key
popd

pushd AS7
scion-pki certificate create --profile=cp-root <(echo '{"isd_as": "16-ffaa:1:2", "common_name": "16-ffaa:1:2 cp root cert"}') cp-root.pem cp-root.key
popd

pushd AS8
scion-pki certificate create --profile=sensitive-voting <(echo '{"isd_as": "16-ffaa:1:3", "common_name": "16-ffaa:1:3 sensitive voting cert"}') sensitive-voting.pem sensitive-voting.key
scion-pki certificate create --profile=regular-voting <(echo '{"isd_as": "16-ffaa:1:3", "common_name": "16-ffaa:1:3 regular voting cert"}') regular-voting.pem regular-voting.key
popd

# Create the TRC
mkdir -p tmp
cat <<EOF > trc-B1-S1-pld.tmpl
isd = 16
description = "Demo ISD 16"
serial_version = 1
base_version = 1
voting_quorum = 2

core_ases = ["ffaa:1:1", "ffaa:1:2", "ffaa:1:3"]
authoritative_ases = ["ffaa:1:1", "ffaa:1:2", "ffaa:1:3"]
cert_files = ["AS6/sensitive-voting.pem", "AS6/regular-voting.pem", "AS6/cp-root.pem", "AS7/cp-root.pem", "AS8/sensitive-voting.pem", "AS8/regular-voting.pem"]

[validity]
not_before = $(date +%s)
validity = "365d"
EOF

scion-pki trc payload --out=tmp/ISD16-B1-S1.pld.der --template trc-B1-S1-pld.tmpl
rm trc-B1-S1-pld.tmpl

# Sign and bundle the TRC
scion-pki trc sign tmp/ISD16-B1-S1.pld.der AS6/sensitive-voting.{pem,key} --out tmp/ISD16-B1-S1.AS6-sensitive.trc
scion-pki trc sign tmp/ISD16-B1-S1.pld.der AS6/regular-voting.{pem,key} --out tmp/ISD16-B1-S1.AS6-regular.trc
scion-pki trc sign tmp/ISD16-B1-S1.pld.der AS8/sensitive-voting.{pem,key} --out tmp/ISD16-B1-S1.AS8-sensitive.trc
scion-pki trc sign tmp/ISD16-B1-S1.pld.der AS8/regular-voting.{pem,key} --out tmp/ISD16-B1-S1.AS8-regular.trc

scion-pki trc combine tmp/ISD16-B1-S1.AS{6,8}-{sensitive,regular}.trc --payload tmp/ISD16-B1-S1.pld.der --out ISD16-B1-S1.trc
rm tmp -r

# Create CA key and certificate for issuing ASes
pushd AS6
scion-pki certificate create --profile=cp-ca <(echo '{"isd_as": "16-ffaa:1:1", "common_name": "16-ffaa:1:1 CA cert"}') cp-ca.pem cp-ca.key --ca cp-root.pem --ca-key cp-root.key
popd
pushd AS7
scion-pki certificate create --profile=cp-ca <(echo '{"isd_as": "16-ffaa:1:2", "common_name": "16-ffaa:1:2 CA cert"}') cp-ca.pem cp-ca.key --ca cp-root.pem --ca-key cp-root.key
popd

# Create AS key and certificate chains
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "16-ffaa:1:1", "common_name": "16-ffaa:1:1 AS cert"}') AS6/cp-as.pem AS6/cp-as.key --ca AS6/cp-ca.pem --ca-key AS6/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "16-ffaa:1:2", "common_name": "16-ffaa:1:2 AS cert"}') AS7/cp-as.pem AS7/cp-as.key --ca AS7/cp-ca.pem --ca-key AS7/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "16-ffaa:1:3", "common_name": "16-ffaa:1:3 AS cert"}') AS8/cp-as.pem AS8/cp-as.key --ca AS6/cp-ca.pem --ca-key AS6/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "16-ffaa:1:4", "common_name": "16-ffaa:1:4 AS cert"}') AS9/cp-as.pem AS9/cp-as.key --ca AS6/cp-ca.pem --ca-key AS6/cp-ca.key --bundle
scion-pki certificate create --profile=cp-as <(echo '{"isd_as": "16-ffaa:1:5", "common_name": "16-ffaa:1:5 AS cert"}') AS10/cp-as.pem AS10/cp-as.key --ca AS7/cp-ca.pem --ca-key AS7/cp-ca.key --bundle

echo 'copying to shared folder'
cp -r /tmp/tutorial-scion-certs-isd02 /shared/