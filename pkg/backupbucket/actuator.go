// SPDX-FileCopyrightText: 2026 Mitja Martini
//
// SPDX-License-Identifier: Apache-2.0

// Package backupbucket implements the Gardener BackupBucket Actuator for the
// "hcloud" provider type, backed by Hetzner Object Storage (S3-compatible).
package backupbucket

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/mitja/hcloud-gardener-backupbucket/pkg/hcloud"
)

type actuator struct {
	client client.Client
}

// NewActuator creates a BackupBucket Actuator backed by Hetzner Object Storage.
func NewActuator(mgr manager.Manager) backupbucket.Actuator {
	return &actuator{client: mgr.GetClient()}
}

// Reconcile ensures the object-storage bucket exists and publishes the
// credentials etcd-backup-restore needs via the generated secret.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	creds, err := a.credentials(ctx, bb)
	if err != nil {
		return err
	}
	hc, err := hcloud.NewClient(creds)
	if err != nil {
		return err
	}

	log.Info("Ensuring backup bucket", "bucket", bb.Name, "region", creds.Region, "endpoint", creds.Endpoint)
	if err := hc.EnsureBucket(ctx, bb.Name); err != nil {
		return fmt.Errorf("could not ensure bucket %q: %w", bb.Name, err)
	}

	return a.ensureGeneratedSecret(ctx, bb, creds)
}

// Delete removes the generated secret and the object-storage bucket.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	if ref := bb.Status.GeneratedSecretRef; ref != nil {
		if err := kutil.DeleteObject(ctx, a.client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace}}); err != nil {
			return fmt.Errorf("could not delete generated secret: %w", err)
		}
	}

	creds, err := a.credentials(ctx, bb)
	if err != nil {
		return err
	}
	hc, err := hcloud.NewClient(creds)
	if err != nil {
		return err
	}

	log.Info("Deleting backup bucket", "bucket", bb.Name)
	if err := hc.DeleteBucket(ctx, bb.Name); err != nil {
		return fmt.Errorf("could not delete bucket %q: %w", bb.Name, err)
	}
	return nil
}

func (a *actuator) credentials(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) (*hcloud.Credentials, error) {
	secret, err := kutil.GetSecretByReference(ctx, a.client, &bb.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get secret %s/%s: %w", bb.Spec.SecretRef.Namespace, bb.Spec.SecretRef.Name, err)
	}
	creds, err := hcloud.CredentialsFromSecret(secret.Data)
	if err != nil {
		return nil, gardencorev1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorConfigurationProblem)
	}
	return creds, nil
}

func (a *actuator) ensureGeneratedSecret(ctx context.Context, bb *extensionsv1alpha1.BackupBucket, creds *hcloud.Credentials) error {
	generatedSecret := &corev1.Secret{ObjectMeta: backupbucket.GeneratedSecretObjectMeta(bb)}
	if _, err := controllerutil.CreateOrUpdate(ctx, a.client, generatedSecret, func() error {
		generatedSecret.Type = corev1.SecretTypeOpaque
		generatedSecret.Data = hcloud.ETCDBackupSecretData(creds)
		return nil
	}); err != nil {
		return fmt.Errorf("could not create or update generated secret: %w", err)
	}

	patch := client.MergeFrom(bb.DeepCopy())
	bb.Status.GeneratedSecretRef = &corev1.SecretReference{
		Name:      generatedSecret.Name,
		Namespace: generatedSecret.Namespace,
	}
	return a.client.Status().Patch(ctx, bb, patch)
}
