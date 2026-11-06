package bedrockiam

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

const signingService = "bedrock"

const credentialsExpiryWindow = time.Minute

type Target struct {
	Region      string
	RoleArn     string
	ExternalId  string
	SessionName string
	SessionTags map[string]string
}

func (t Target) sessionTags() []types.Tag {
	keys := make([]string, 0, len(t.SessionTags))
	for k := range t.SessionTags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tags := make([]types.Tag, 0, len(keys))
	for _, k := range keys {
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(t.SessionTags[k])})
	}

	return tags
}

func cacheKey(t Target, tags []types.Tag) string {
	parts := []string{t.Region, t.RoleArn, t.ExternalId, t.SessionName}
	for _, tag := range tags {
		parts = append(parts, aws.ToString(tag.Key)+"="+aws.ToString(tag.Value))
	}

	return strings.Join(parts, "\x00")
}

type Signer struct {
	newStsClient func(region string) stscreds.AssumeRoleAPIClient
	signer       *v4.Signer

	mu     sync.Mutex
	caches map[string]*aws.CredentialsCache
}

func NewSigner(base aws.Config) *Signer {
	return newSigner(func(region string) stscreds.AssumeRoleAPIClient {
		return sts.NewFromConfig(base, func(o *sts.Options) {
			o.Region = region
		})
	})
}

func newSigner(newStsClient func(region string) stscreds.AssumeRoleAPIClient) *Signer {
	return &Signer{
		newStsClient: newStsClient,
		signer:       v4.NewSigner(),
		caches:       map[string]*aws.CredentialsCache{},
	}
}

func (s *Signer) Sign(ctx context.Context, req *http.Request, body []byte, target Target) error {
	creds, err := s.credentials(target).Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("assume role %s: %w", target.RoleArn, err)
	}

	sum := sha256.Sum256(body)
	return s.signer.SignHTTP(ctx, creds, req, hex.EncodeToString(sum[:]), signingService, target.Region, time.Now())
}

func (s *Signer) credentials(target Target) *aws.CredentialsCache {
	tags := target.sessionTags()
	key := cacheKey(target, tags)

	s.mu.Lock()
	defer s.mu.Unlock()

	if cache, ok := s.caches[key]; ok {
		return cache
	}

	provider := stscreds.NewAssumeRoleProvider(s.newStsClient(target.Region), target.RoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = target.SessionName
		if len(tags) != 0 {
			o.Tags = tags
		}
		if len(target.ExternalId) != 0 {
			o.ExternalID = aws.String(target.ExternalId)
		}
	})

	cache := aws.NewCredentialsCache(provider, func(o *aws.CredentialsCacheOptions) {
		o.ExpiryWindow = credentialsExpiryWindow
	})
	s.caches[key] = cache

	return cache
}
