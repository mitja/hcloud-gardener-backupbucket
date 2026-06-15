#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Mitja Martini
#
# SPDX-License-Identifier: Apache-2.0
#
# Generates a Gardener ControllerDeployment + ControllerRegistration for the
# backupbucket-hcloud extension by gzip+base64-encoding the Helm chart into the
# ControllerDeployment's helm.rawChart field.
#
# Usage:
#   hack/generate-controller-registration.sh [CHART_DIR] [OUTPUT_FILE] [IMAGE_TAG]

set -o errexit
set -o nounset
set -o pipefail

NAME="backupbucket-hcloud"
# Provider type MUST be one etcd-druid maps to an S3 store (it rejects "hcloud" as an
# unsupported storage provider). "S3" = the protocol; impl is still Hetzner Object Storage.
PROVIDER_TYPE="S3"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${1:-${REPO_ROOT}/charts/gardener-extension-backupbucket-hcloud}"
OUTPUT_FILE="${2:-${REPO_ROOT}/example/controller-registration.yaml}"
IMAGE_TAG="${3:-latest}"

if [[ ! -d "${CHART_DIR}" ]]; then
  echo "chart directory ${CHART_DIR} does not exist" >&2
  exit 1
fi

CHART_PARENT="$(cd "${CHART_DIR}/.." && pwd)"
CHART_BASE="$(basename "${CHART_DIR}")"

if command -v gtar >/dev/null 2>&1; then
  RAW_CHART="$(
    gtar --sort=name --mtime='2026-01-01 00:00:00 UTC' \
         --owner=0 --group=0 --numeric-owner \
         -C "${CHART_PARENT}" -czf - "${CHART_BASE}" | base64 | tr -d '\n'
  )"
else
  RAW_CHART="$(
    COPYFILE_DISABLE=1 tar --exclude '._*' -C "${CHART_PARENT}" -czf - "${CHART_BASE}" | base64 | tr -d '\n'
  )"
fi

mkdir -p "$(dirname "${OUTPUT_FILE}")"

cat > "${OUTPUT_FILE}" <<EOF
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: ${NAME}
helm:
  rawChart: ${RAW_CHART}
  values:
    image:
      tag: ${IMAGE_TAG}
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: ${NAME}
  annotations:
    security.gardener.cloud/pod-security-enforce: baseline
spec:
  deployment:
    deploymentRefs:
    - name: ${NAME}
    policy: OnDemand
  resources:
  - kind: BackupBucket
    type: ${PROVIDER_TYPE}
  - kind: BackupEntry
    type: ${PROVIDER_TYPE}
EOF

echo "Wrote ${OUTPUT_FILE} (image tag ${IMAGE_TAG})"
