// SPDX-FileCopyrightText: 2026 Mitja Martini
//
// SPDX-License-Identifier: Apache-2.0

// Package hcloud provides a thin S3 client for Hetzner Object Storage (which is
// S3-compatible) plus helpers to read credentials from a Kubernetes Secret.
package hcloud

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Secret data keys this extension reads. Both camelCase and UPPER_SNAKE_CASE are
// accepted so the same secret works for tooling that uses either convention.
const (
	keyAccessKeyID     = "accessKeyID"
	keyAccessKeyIDAlt  = "ACCESS_KEY_ID"
	keySecretKey       = "secretAccessKey"
	keySecretKeyAlt    = "SECRET_ACCESS_KEY"
	keyRegion          = "region"
	keyRegionAlt       = "REGION"
	keyEndpoint        = "endpoint"
	keyEndpointAlt     = "ENDPOINT"
	defaultRegion      = "nbg1"
)

// Credentials holds the resolved Hetzner Object Storage S3 credentials.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Endpoint        string // host[:port], no scheme — e.g. nbg1.your-objectstorage.com
}

// CredentialsFromSecret extracts the S3 credentials from a Secret's data.
func CredentialsFromSecret(data map[string][]byte) (*Credentials, error) {
	get := func(primary, alt string) string {
		if v, ok := data[primary]; ok {
			return string(v)
		}
		return string(data[alt])
	}

	c := &Credentials{
		AccessKeyID:     get(keyAccessKeyID, keyAccessKeyIDAlt),
		SecretAccessKey: get(keySecretKey, keySecretKeyAlt),
		Region:          get(keyRegion, keyRegionAlt),
		Endpoint:        get(keyEndpoint, keyEndpointAlt),
	}
	if c.Region == "" {
		c.Region = defaultRegion
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return nil, fmt.Errorf("secret is missing %q/%q and %q/%q", keyAccessKeyID, keyAccessKeyIDAlt, keySecretKey, keySecretKeyAlt)
	}
	if c.Endpoint == "" {
		return nil, fmt.Errorf("secret is missing the %q (e.g. nbg1.your-objectstorage.com)", keyEndpoint)
	}
	return c, nil
}

// ETCDBackupSecretData returns the credential data in the canonical key layout
// expected by etcd-backup-restore's S3 store provider.
func ETCDBackupSecretData(c *Credentials) map[string][]byte {
	return map[string][]byte{
		"accessKeyID":     []byte(c.AccessKeyID),
		"secretAccessKey": []byte(c.SecretAccessKey),
		"region":          []byte(c.Region),
		"endpoint":        []byte(c.Endpoint),
	}
}

// Client is the minimal object-storage surface this extension needs.
type Client interface {
	// EnsureBucket creates the bucket if it does not exist (idempotent).
	EnsureBucket(ctx context.Context, name string) error
	// DeleteBucket empties and deletes the bucket (idempotent).
	DeleteBucket(ctx context.Context, name string) error
	// DeletePrefix deletes every object under bucket/prefix (idempotent).
	DeletePrefix(ctx context.Context, bucket, prefix string) error
}

type client struct {
	mc     *minio.Client
	region string
}

// NewClient builds an S3 client for Hetzner Object Storage from the credentials.
func NewClient(c *Credentials) (Client, error) {
	mc, err := minio.New(c.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(c.AccessKeyID, c.SecretAccessKey, ""),
		Secure:       true,
		Region:       c.Region,
		BucketLookup: minio.BucketLookupAuto,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create S3 client: %w", err)
	}
	return &client{mc: mc, region: c.Region}, nil
}

func (c *client) EnsureBucket(ctx context.Context, name string) error {
	exists, err := c.mc.BucketExists(ctx, name)
	if err != nil {
		return fmt.Errorf("could not check bucket %q: %w", name, err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, name, minio.MakeBucketOptions{Region: c.region}); err != nil {
		// Race / already-owned: treat as success.
		if exists2, errExists := c.mc.BucketExists(ctx, name); errExists == nil && exists2 {
			return nil
		}
		return fmt.Errorf("could not create bucket %q: %w", name, err)
	}
	return nil
}

func (c *client) DeleteBucket(ctx context.Context, name string) error {
	exists, err := c.mc.BucketExists(ctx, name)
	if err != nil {
		return fmt.Errorf("could not check bucket %q: %w", name, err)
	}
	if !exists {
		return nil
	}
	if err := c.DeletePrefix(ctx, name, ""); err != nil {
		return err
	}
	if err := c.mc.RemoveBucket(ctx, name); err != nil {
		return fmt.Errorf("could not remove bucket %q: %w", name, err)
	}
	return nil
}

func (c *client) DeletePrefix(ctx context.Context, bucket, prefix string) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range c.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if obj.Err != nil {
				continue
			}
			objectsCh <- obj
		}
	}()

	for rErr := range c.mc.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if rErr.Err != nil {
			return fmt.Errorf("could not delete %q in bucket %q: %w", rErr.ObjectName, bucket, rErr.Err)
		}
	}
	return nil
}
