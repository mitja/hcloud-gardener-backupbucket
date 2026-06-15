# SPDX-FileCopyrightText: 2026 Mitja Martini
#
# SPDX-License-Identifier: Apache-2.0

# ---- build stage ----
FROM golang:1.24 AS builder

WORKDIR /workspace

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /gardener-extension-backupbucket-hcloud \
      ./cmd/gardener-extension-backupbucket-hcloud

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /gardener-extension-backupbucket-hcloud /gardener-extension-backupbucket-hcloud

USER 65532:65532
ENTRYPOINT ["/gardener-extension-backupbucket-hcloud"]
