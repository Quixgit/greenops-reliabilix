package domain

import (
	"fmt"
	"strings"
)

// ExportConfig locates a customer's FOCUS data export (AWS Data Exports) in S3. Layout written by AWS:
//
//	s3://<Bucket>/<Prefix>/<Name>/metadata/BILLING_PERIOD=YYYY-MM/<Name>-Manifest.json
//	s3://<Bucket>/<Prefix>/<Name>/data/BILLING_PERIOD=YYYY-MM/<Name>-<n>.csv.gz
//
// These are references, never credentials: access comes from the customer's read-only role.
type ExportConfig struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"` // may be empty (export written at the bucket root)
	Name   string `json:"name"`   // the export's name
	Region string `json:"region"` // region of the bucket
}

// Validate rejects anything that could steer the platform to read outside the intended export.
func (e ExportConfig) Validate() error {
	if !bucketRe.MatchString(e.Bucket) || strings.Contains(e.Bucket, "..") {
		return fmt.Errorf("%w: invalid export bucket name", ErrInvalidConnection)
	}
	if !exportPartRe.MatchString(e.Name) || e.Name == "." || e.Name == ".." {
		return fmt.Errorf("%w: invalid export name", ErrInvalidConnection)
	}
	if !regionRe.MatchString(e.Region) {
		return fmt.Errorf("%w: invalid export bucket region", ErrInvalidConnection)
	}
	if e.Prefix != "" {
		if len(e.Prefix) > 200 || strings.HasPrefix(e.Prefix, "/") || strings.HasSuffix(e.Prefix, "/") {
			return fmt.Errorf("%w: export prefix must not start or end with a slash", ErrInvalidConnection)
		}
		for _, seg := range strings.Split(e.Prefix, "/") {
			if seg == "" || seg == "." || seg == ".." || !exportPartRe.MatchString(seg) {
				return fmt.Errorf("%w: invalid export prefix", ErrInvalidConnection)
			}
		}
	}
	return nil
}

// Root is the key prefix under which the export lives ("<prefix>/<name>/").
func (e ExportConfig) Root() string {
	if e.Prefix == "" {
		return e.Name + "/"
	}
	return e.Prefix + "/" + e.Name + "/"
}
