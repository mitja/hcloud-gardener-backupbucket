// SPDX-FileCopyrightText: 2026 Mitja Martini
//
// SPDX-License-Identifier: Apache-2.0

// Package backupentry implements the Gardener BackupEntry delegate for the
// "hcloud" provider type. Reconcile/Restore/Migrate are handled by the generic
// actuator (which writes the etcd-backup-restore secret from GetETCDSecretData);
// Delete removes the entry's prefix from the bucket.
package backupentry

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/mitja/hcloud-gardener-backupbucket/pkg/hcloud"
)

type delegate struct {
	client client.Client
}

func newDelegate(mgr manager.Manager) genericactuator.BackupEntryDelegate {
	return &delegate{client: mgr.GetClient()}
}

// GetETCDSecretData normalises the bucket's S3 credentials into the canonical key
// layout etcd-backup-restore's S3 store provider expects.
func (d *delegate) GetETCDSecretData(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	creds, err := hcloud.CredentialsFromSecret(backupSecretData)
	if err != nil {
		return nil, gardencorev1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorConfigurationProblem)
	}
	out := make(map[string][]byte, len(backupSecretData)+4)
	for k, v := range backupSecretData {
		out[k] = v
	}
	for k, v := range hcloud.ETCDBackupSecretData(creds) {
		out[k] = v
	}
	return out, nil
}

// Delete removes all objects under the entry's prefix in the bucket.
func (d *delegate) Delete(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	secret, err := kutil.GetSecretByReference(ctx, d.client, &be.Spec.SecretRef)
	if err != nil {
		return fmt.Errorf("could not get secret %s/%s: %w", be.Spec.SecretRef.Namespace, be.Spec.SecretRef.Name, err)
	}
	creds, err := hcloud.CredentialsFromSecret(secret.Data)
	if err != nil {
		return gardencorev1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorConfigurationProblem)
	}
	hc, err := hcloud.NewClient(creds)
	if err != nil {
		return err
	}

	entryName := strings.TrimPrefix(be.Name, v1beta1constants.BackupSourcePrefix+"-")
	prefix := entryName + "/"
	log.Info("Deleting backup entry prefix", "bucket", be.Spec.BucketName, "prefix", prefix)
	if err := hc.DeletePrefix(ctx, be.Spec.BucketName, prefix); err != nil {
		return fmt.Errorf("could not delete prefix %q in bucket %q: %w", prefix, be.Spec.BucketName, err)
	}
	return nil
}
