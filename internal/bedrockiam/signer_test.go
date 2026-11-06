package bedrockiam

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type fakeSts struct {
	mu     sync.Mutex
	calls  []*sts.AssumeRoleInput
	region string
	expiry time.Time
}

func (f *fakeSts) AssumeRole(_ context.Context, in *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, in)

	return &sts.AssumeRoleOutput{
		Credentials: &types.Credentials{
			AccessKeyId:     aws.String("ASIATEST"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("session-token"),
			Expiration:      aws.Time(f.expiry),
		},
	}, nil
}

func (f *fakeSts) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.calls)
}

func newTestSigner(fake *fakeSts) *Signer {
	return newSigner(func(region string) stscreds.AssumeRoleAPIClient {
		fake.region = region
		return fake
	})
}

func testTarget() Target {
	return Target{
		Region:      "us-east-1",
		RoleArn:     "arn:aws:iam::123456789012:role/bricks-opencode",
		ExternalId:  "ext",
		SessionName: "bricksllm-setting-1",
		SessionTags: map[string]string{"service": "opencode", "env": "stand"},
	}
}

func TestSignAddsSigV4HeadersWithSessionToken(t *testing.T) {
	fake := &fakeSts{expiry: time.Now().Add(time.Hour)}
	s := newTestSigner(fake)

	body := []byte(`{"model":"m"}`)
	req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/anthropic/v1/messages", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	if err := s.Sign(context.Background(), req, body, testTarget()); err != nil {
		t.Fatalf("sign: %v", err)
	}

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=ASIATEST/") || !strings.Contains(auth, "/us-east-1/bedrock/aws4_request") {
		t.Fatalf("unexpected authorization header: %s", auth)
	}

	if !strings.Contains(auth, "anthropic-version") || !strings.Contains(auth, "content-type") {
		t.Fatalf("expected forwarded headers to be signed: %s", auth)
	}

	if req.Header.Get("X-Amz-Security-Token") != "session-token" {
		t.Fatalf("expected session token header, got %q", req.Header.Get("X-Amz-Security-Token"))
	}

	if req.Header.Get("X-Amz-Date") == "" {
		t.Fatal("expected X-Amz-Date header")
	}
}

func TestSignAssumesRoleWithTargetParameters(t *testing.T) {
	fake := &fakeSts{expiry: time.Now().Add(time.Hour)}
	s := newTestSigner(fake)

	req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1/responses", nil)
	if err := s.Sign(context.Background(), req, nil, testTarget()); err != nil {
		t.Fatalf("sign: %v", err)
	}

	if fake.region != "us-east-1" {
		t.Fatalf("expected sts client in target region, got %q", fake.region)
	}

	in := fake.calls[0]
	if aws.ToString(in.RoleArn) != "arn:aws:iam::123456789012:role/bricks-opencode" {
		t.Fatalf("unexpected role arn %q", aws.ToString(in.RoleArn))
	}

	if aws.ToString(in.RoleSessionName) != "bricksllm-setting-1" {
		t.Fatalf("unexpected session name %q", aws.ToString(in.RoleSessionName))
	}

	if aws.ToString(in.ExternalId) != "ext" {
		t.Fatalf("unexpected external id %q", aws.ToString(in.ExternalId))
	}

	if len(in.Tags) != 2 || aws.ToString(in.Tags[0].Key) != "env" || aws.ToString(in.Tags[1].Key) != "service" || aws.ToString(in.Tags[1].Value) != "opencode" {
		t.Fatalf("unexpected session tags %+v", in.Tags)
	}
}

func TestSignReusesCachedCredentials(t *testing.T) {
	fake := &fakeSts{expiry: time.Now().Add(time.Hour)}
	s := newTestSigner(fake)

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/anthropic/v1/messages", nil)
		if err := s.Sign(context.Background(), req, nil, testTarget()); err != nil {
			t.Fatalf("sign: %v", err)
		}
	}

	if fake.callCount() != 1 {
		t.Fatalf("expected one AssumeRole call, got %d", fake.callCount())
	}
}

func TestSignRefreshesCredentialsCloseToExpiry(t *testing.T) {
	fake := &fakeSts{expiry: time.Now().Add(30 * time.Second)}
	s := newTestSigner(fake)

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/anthropic/v1/messages", nil)
		if err := s.Sign(context.Background(), req, nil, testTarget()); err != nil {
			t.Fatalf("sign: %v", err)
		}
	}

	if fake.callCount() != 2 {
		t.Fatalf("expected credentials inside the expiry window to be refreshed, got %d AssumeRole calls", fake.callCount())
	}
}

func TestSignSeparatesCachesPerTarget(t *testing.T) {
	fake := &fakeSts{expiry: time.Now().Add(time.Hour)}
	s := newTestSigner(fake)

	other := testTarget()
	other.SessionTags = map[string]string{"service": "zeus"}

	for _, target := range []Target{testTarget(), other, testTarget()} {
		req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/anthropic/v1/messages", nil)
		if err := s.Sign(context.Background(), req, nil, target); err != nil {
			t.Fatalf("sign: %v", err)
		}
	}

	if fake.callCount() != 2 {
		t.Fatalf("expected one AssumeRole call per distinct target, got %d", fake.callCount())
	}
}
