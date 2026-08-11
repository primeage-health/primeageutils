package gcs

import (
	"fmt"
	"net/url"
	"time"

	"cloud.google.com/go/storage"
)

// SignedUploadURL returns a URL the caller may PUT one object to.
//
// contentType is signed into the URL, so the PUT must carry exactly that
// Content-Type header or Google refuses it. That pins the declared type to the
// one the issuing service authorised, which a bare signed URL would not.
//
// What it cannot pin is size: a V4 signed PUT has no content-length condition,
// so the only bound on how much a holder can write is how long the URL lives.
// Keep ttl short. A signed POST policy is the mechanism that carries a
// content-length-range, and is what to reach for if a hard cap is needed.
func (s *Storage) SignedUploadURL(object, contentType string, ttl time.Duration) (string, time.Time, error) {
	expiry := time.Now().Add(ttl)
	signed, err := s.client.Bucket(s.bucket).SignedURL(object, &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		ContentType: contentType,
		Expires:     expiry,
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("unable to sign upload url: %w", err)
	}
	return signed, expiry, nil
}

// SignedDownloadURL returns a URL the caller may GET one object from.
//
// downloadName is what the browser saves the file as. Objects are stored under
// a generated key, so without this a download would land on disk named after
// that key rather than after the file the user uploaded.
func (s *Storage) SignedDownloadURL(object, downloadName string, ttl time.Duration) (string, time.Time, error) {
	expiry := time.Now().Add(ttl)

	query := url.Values{}
	if downloadName != "" {
		query.Set("response-content-disposition", `attachment; filename="`+downloadName+`"`)
	}

	signed, err := s.client.Bucket(s.bucket).SignedURL(object, &storage.SignedURLOptions{
		Scheme:          storage.SigningSchemeV4,
		Method:          "GET",
		Expires:         expiry,
		QueryParameters: query,
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("unable to sign download url: %w", err)
	}
	return signed, expiry, nil
}
