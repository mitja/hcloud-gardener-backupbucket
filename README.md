# gardener-extension-backupbucket-hcloud

A [Gardener](https://gardener.cloud) extension implementing the **BackupBucket**
and **BackupEntry** contracts for **Hetzner Object Storage** (S3-compatible),
provider type **`hcloud`**. It lets gardenlet/etcd-druid back up the virtual-garden
and shoot etcds to Hetzner Object Storage — the piece `provider-hcloud` does not
ship (no native BackupBucket controller).

Sibling to [`hcloud-gardener-dnsrecord`](../hcloud-gardener-dnsrecord) and built the
same way; pinned to gardener **v1.122.3**.

## What it does

| Resource | Reconcile | Delete |
|---|---|---|
| `BackupBucket` (type `hcloud`) | create the S3 bucket (named after the BackupBucket) + publish a generated secret with the S3 credentials → `status.generatedSecretRef` | delete the generated secret, then empty + delete the bucket |
| `BackupEntry` (type `hcloud`) | (generic actuator) write the per-entry etcd-backup-restore secret from `GetETCDSecretData` | delete the entry's `<entry>/` prefix from the bucket |

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
docker build -t git.paasbox.com/paasbox/hcloud-gardener-backupbucket:dev .
```
CI: `.forgejo/workflows/build.yml` builds + pushes `git.paasbox.com/paasbox/
hcloud-gardener-backupbucket:{<sha>,latest}` on push to `main` (org Actions secrets
`REGISTRY_USER`/`REGISTRY_TOKEN`).

## Install (register in the virtual garden)

```sh
hack/generate-controller-registration.sh   # → example/controller-registration.yaml (chart inlined)
kubectl apply -f example/controller-registration.yaml   # against the virtual garden
```
Because the image is in a **private** registry, set `image.pullPolicy` + an
`imagePullSecrets` in the ControllerDeployment `values` (chart `values.yaml`).

Then enable seed backups, e.g.:
```yaml
# Seed (or the Gardenlet seedConfig)
spec:
  backup:
    provider: hcloud
    region: nbg1
    secretRef:
      name: backup-hcloud
      namespace: garden
```

## ⚠️ Known integration point to verify (etcd-backup-restore provider)

Hetzner Object Storage is S3-compatible, so etcd-backup-restore must use its **S3**
store provider **with the custom `endpoint`**. gardenlet derives the etcd store
provider from the seed backup provider type (`hcloud`); etcd-backup-restore must map
that to the `S3` protocol + honour the `endpoint` from the secret. This extension
supplies the S3 credentials + endpoint in the etcd backup secret (via the BackupEntry
`GetETCDSecretData`), but the provider/endpoint wiring on the etcd-druid /
etcd-backup-restore side should be **validated end-to-end** on first use (it may need
an etcd-druid image-vector / store-provider mapping). See the gardener etcd-backup
docs. This is the main thing to test before relying on it.

## License

Apache-2.0.
