package app

import (
	"context"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
)

type fakeValues map[string]string

func (f fakeValues) Value(_ context.Context, integration, key string) string {
	return f[integration+"/"+key]
}

func TestConsoleValuesWinAndEnvironmentStaysOtherwise(t *testing.T) {
	cfg := config.Config{S3Bucket: "env-bucket", S3Region: "eu-west-1", SentryDSN: "https://env@sentry/1", PlatformAWSAccountID: "111111111111", S3PathStyle: false}
	got := overlayIntegrations(context.Background(), cfg, fakeValues{
		"object_storage/bucket":       "console-bucket",
		"object_storage/path_style":   "true",
		"platform_aws/account_id":     "222222222222",
		"observability/otlp_endpoint": "http://collector:4318",
	})
	if got.S3Bucket != "console-bucket" || !got.S3PathStyle || got.PlatformAWSAccountID != "222222222222" || got.OTLPEndpoint != "http://collector:4318" {
		t.Fatalf("console values must apply: %+v", got)
	}
	if got.S3Region != "eu-west-1" || got.SentryDSN != "https://env@sentry/1" {
		t.Fatalf("fields the console does not set keep the environment's value: %+v", got)
	}
	// An empty console answer never blanks the environment's value.
	same := overlayIntegrations(context.Background(), cfg, fakeValues{})
	if same.S3Bucket != cfg.S3Bucket || same.SentryDSN != cfg.SentryDSN || same.PlatformAWSAccountID != cfg.PlatformAWSAccountID || same.S3PathStyle != cfg.S3PathStyle {
		t.Fatalf("no console values, no change: %+v", same)
	}
}

func TestEnvironmentFallbackCoversEveryStartupField(t *testing.T) {
	cfg := config.Config{PlatformAWSAccountID: "a", PlatformGCPProject: "b", PlatformGCPServiceAccount: "c", S3Bucket: "d", S3Endpoint: "e", S3Region: "f",
		S3PathStyle: true, S3AccessKey: "g", S3SecretKey: "h", SentryDSN: "i", OTLPEndpoint: "j"}
	env := integrationEnv(cfg)
	for _, k := range []string{"platform_aws/account_id", "platform_gcp/project", "platform_gcp/service_account", "object_storage/bucket",
		"object_storage/endpoint", "object_storage/region", "object_storage/path_style", "object_storage/access_key",
		"object_storage/secret_key", "observability/sentry_dsn", "observability/otlp_endpoint"} {
		in, key, _ := splitOnce(k)
		if env(in, key) == "" {
			t.Errorf("%s has no environment fallback", k)
		}
	}
}

func splitOnce(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
