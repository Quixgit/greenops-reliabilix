// Package pricing holds what the cloud price-index clients (awsprice, azureprice, gcpprice) share.
package pricing

import "errors"

// ErrNotOffered says a region has no price for the service, or too few of the basket to compare: the service is not sold there. It is
// an answer, not a failure: the region simply has no index and no estimate, and a job must not be retried over it.
var ErrNotOffered = errors.New("pricing: the service is not offered in this region")
