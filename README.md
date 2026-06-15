# gardener-extension-backupbucket-hcloud

A [Gardener](https://gardener.cloud) extension implementing the **BackupBucket**
and **BackupEntry** contracts for **Hetzner Object Storage** (S3-compatible). It lets
gardenlet/etcd-druid back up the virtual-garden and shoot etcds to Hetzner Object
Storage — the piece `provider-hcloud` does not ship (no native BackupBucket controller).

It registers the provider type **`S3`** (not `hcloud`): etcd-druid feeds the seed/garden
backup provider straight through as the etcd store provider and only accepts a fixed set
(`aws`/`S3`/`stackit`/…), rejecting `hcloud` as an *"unsupported storage provider"*. `S3`
names the **protocol**; the implementation underneath is still Hetzner Object Storage
(custom endpoint via the secret). This is the Hetzner/`hcloud` S3-backup extension.

Sibling to [`hcloud-gardener-dnsrecord`](../hcloud-gardener-dnsrecord) and built the
same way; pinned to gardener **v1.122.3**.

## What it does

| Resource | Reconcile | Delete |
|---|---|---|
| `BackupBucket` (type `S3`) | create the S3 bucket (named after the BackupBucket) + publish a generated secret with the S3 credentials → `status.generatedSecretRef` | delete the generated secret, then empty + delete the bucket |
| `BackupEntry` (type `S3`) | (generic actuator) write the per-entry etcd-backup-restore secret from `GetETCDSecretData` | delete the entry's `<entry>/` prefix from the bucket |

The S3 client is [minio-go](https://github.com/minio/minio-go) against
`https://<endpoint>` (TLS, V4 signing).

## Backup secret

The secret referenced by `Seed.spec.backup.secretRef` (Hetzner Object Storage
credentials) must contain — `camelCase` or `UPPER_SNAKE_CASE` accepted:

| key | required | notes |
|---|---|---|
| `accessKeyID` / `ACCESS_KEY_ID` | yes | Hetzner Object Storage S3 access key |
| `secretAccessKey` / `SECRET_ACCESS_KEY` | yes | … secret key |
| `endpoint` / `ENDPOINT` | yes | host, no scheme — e.g. `nbg1.your-objectstorage.com` |
| `region` / `REGION` | no | default `nbg1` |

The extension re-emits these (canonical keys `accessKeyID`/`secretAccessKey`/
`region`/`endpoint`) into the generated bucket secret and the per-entry
etcd-backup-restore secret.

## Build

```sh
go build ./...                                   # compile
docker build -t ghcr.io/mitja/hcloud-gardener-backupbucket:dev .
```
CI — the image is built + published to **both** registries (policy: every image is kept on
Forgejo so the platform never depends on GHCR uptime; public repos also publish to GHCR):
- **GHCR (public pull source)** — `.github/workflows/release.yml` (GitHub Actions) runs on
  a `v*` tag: `go vet`/`go test`, builds + pushes `ghcr.io/mitja/hcloud-gardener-backupbucket:
  {<tag>,latest}` (what the seed pulls; mirrors `hcloud-gardener-dnsrecord`) via the
  built-in `GITHUB_TOKEN` (no PAT), and publishes a GitHub release with the generated
  `controller-registration.yaml`.
- **Forgejo (always-available fallback)** — `.forgejo/workflows/build.yml` (Forgejo
  Actions) runs on push to `main`/tags and pushes
  `git.paasbox.com/paasbox/hcloud-gardener-backupbucket:{<sha>,latest}`; secrets
  `REGISTRY_USER`/`REGISTRY_TOKEN`. Repoint `image.repository` here to fail over.

The repo lives on **both** git hosts (public GitHub + private Forgejo); push commits and
tags to both remotes so both CIs build.

## Install (register in the virtual garden)

```sh
hack/generate-controller-registration.sh   # → example/controller-registration.yaml (chart inlined)
kubectl apply -f example/controller-registration.yaml   # against the virtual garden
```
The chart defaults to the **public** `ghcr.io/mitja/hcloud-gardener-backupbucket` image,
so no pull secret is needed. To pull from the private Forgejo registry instead, override
`image.repository` and set `imagePullSecrets` in the ControllerDeployment `values`.

Then enable seed backups, e.g.:
```yaml
# Seed (or the Gardenlet seedConfig)
spec:
  backup:
    provider: S3            # NOT "hcloud" — see the note below; etcd-druid requires this
    region: nbg1
    secretRef:
      name: backup-hcloud
      namespace: garden
```

## Why the provider type is `S3`, not `hcloud`

etcd-druid (v0.30.1, bundled with gardener v1.122.3) derives the etcd-backup-restore
store provider **directly** from the seed/garden backup provider — `etcd.go` does
`StorageProvider(backupConfig.Provider)` with **no translation** — and its
`storageProviderFromInfraProvider` map only accepts a fixed set:
`aws`/`S3`, `stackit`→S3, `azure`/`ABS`, `gcp`/`GCS`, `alicloud`/`OSS`,
`openstack`/`Swift`, `dell`/`ECS`, `openshift`/`OCS`, `Local`. Anything else (e.g.
`hcloud`) → **`"unsupported storage provider"`** and etcd backup never configures.

So this extension registers the type **`S3`**, which druid maps to its S3 snapstore.
Hetzner-specificity comes entirely through the **secret**: the BackupEntry
`GetETCDSecretData` supplies `accessKeyID`/`secretAccessKey`/`region`/`endpoint`, and the
S3 snapstore honours the custom `endpoint` (`nbg1.your-objectstorage.com`). Verified by
reading the gardener/etcd-druid source; confirm end-to-end on first enablement that a
snapshot lands in the bucket.

## License

Apache-2.0.
