// Package gcs is the Google Cloud Storage adapter the services share.
//
// It hands out pre-signed URLs rather than moving bytes. The caller PUTs to and
// GETs from GCS directly, so an object never occupies a service's memory or its
// request timeout, and an upload from a phone on a poor link is retried against
// Google rather than against us.
package gcs

import (
	"context"
	"errors"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// Environment this package reads. GCS_BUCKET is required; the two credential
// variables are alternatives, and with neither set the client falls back to
// application default credentials.
const (
	EnvBucket          = "GCS_BUCKET"
	EnvCredentialsJSON = "GCS_CREDENTIALS_JSON"
	EnvKeyFile         = "GCS_KEY_FILE"
)

// ErrObjectNotFound is returned for an object that is not in the bucket.
var ErrObjectNotFound = errors.New("object not found")

// Storage is a client bound to one bucket.
type Storage struct {
	client *storage.Client
	bucket string
}

// New opens a client for the bucket named by GCS_BUCKET.
//
// Signing a URL needs an RSA private key, which application default credentials
// on a workload identity do not carry — that path can only sign by calling IAM,
// and the call needs roles/iam.serviceAccountTokenCreator on the running service
// account. A mounted service-account key is the configuration that works without
// that grant, so GCS_CREDENTIALS_JSON or GCS_KEY_FILE is the expected deployment
// and the fallback is for local development.
func New(ctx context.Context) (*Storage, error) {
	bucket := os.Getenv(EnvBucket)
	if bucket == "" {
		return nil, fmt.Errorf("%s is not set", EnvBucket)
	}

	var opts []option.ClientOption
	if credentials := os.Getenv(EnvCredentialsJSON); credentials != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(credentials)))
	} else if keyFile := os.Getenv(EnvKeyFile); keyFile != "" {
		opts = append(opts, option.WithCredentialsFile(keyFile))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create gcs client: %w", err)
	}

	return &Storage{client: client, bucket: bucket}, nil
}

// Bucket is the bucket every object of this client lives in.
func (s *Storage) Bucket() string { return s.bucket }

// Close releases the client's connections.
func (s *Storage) Close() error { return s.client.Close() }

// Exists reports whether an object is in the bucket.
func (s *Storage) Exists(ctx context.Context, object string) (bool, error) {
	_, err := s.client.Bucket(s.bucket).Object(object).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("unable to read object attributes: %w", err)
	}
	return true, nil
}
