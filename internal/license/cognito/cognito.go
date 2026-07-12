// Package cognito wraps the AWS Cognito user-pool admin operations the
// management portal needs (group management, native user creation, IdP
// provisioning). The narrow Admin interface lets orchestration code be tested
// against a fake — we validate our call construction and idempotency logic, not
// AWS itself (the IQ/OQ boundary).
package cognito

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// defaultRoleSessionName labels the STS session when none is configured.
const defaultRoleSessionName = "janus-license-server"

// Admin is the subset of Cognito user-pool admin operations the portal uses.
// *Client is the production implementation; tests use a fake.
type Admin interface {
	// EnsureGroup idempotently creates a group (no-op if it already exists).
	EnsureGroup(ctx context.Context, group string) error
	// AddUserToGroup adds a user to a group (idempotent in Cognito).
	AddUserToGroup(ctx context.Context, username, group string) error
	// EnsureUser idempotently creates a native (non-federated) Cognito user and
	// returns its sub; if the user already exists, the existing sub is returned.
	EnsureUser(ctx context.Context, email string) (sub string, err error)
	// RemoveUserFromGroup removes a user from a group (used for demote/offboard).
	RemoveUserFromGroup(ctx context.Context, username, group string) error
	// DisableUser disables a user so they can no longer authenticate (offboard).
	DisableUser(ctx context.Context, username string) error
	// CreateIdentityProvider provisions an OIDC IdP in the pool (SSO setup).
	CreateIdentityProvider(ctx context.Context, idp IdentityProvider) error
}

// IdentityProvider is the minimum OIDC configuration needed to register a
// customer's identity provider in the Cognito user pool.
type IdentityProvider struct {
	Name         string            // Cognito provider name (<= 32 chars)
	OIDCIssuer   string            // customer IdP issuer URL (Cognito auto-discovers endpoints)
	ClientID     string            // customer IdP client id
	ClientSecret string            // customer IdP client secret
	Scopes       string            // space-delimited (e.g. "openid email profile")
	AttributeMap map[string]string // IdP claim -> Cognito attribute
}

// cognitoAPI is the subset of the AWS SDK client we depend on. *cognitoidentityprovider.Client
// satisfies it; tests inject a mock.
type cognitoAPI interface {
	CreateGroup(ctx context.Context, in *cognitoidentityprovider.CreateGroupInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.CreateGroupOutput, error)
	AdminAddUserToGroup(ctx context.Context, in *cognitoidentityprovider.AdminAddUserToGroupInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminAddUserToGroupOutput, error)
	AdminCreateUser(ctx context.Context, in *cognitoidentityprovider.AdminCreateUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error)
	AdminGetUser(ctx context.Context, in *cognitoidentityprovider.AdminGetUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminGetUserOutput, error)
	AdminRemoveUserFromGroup(ctx context.Context, in *cognitoidentityprovider.AdminRemoveUserFromGroupInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminRemoveUserFromGroupOutput, error)
	AdminDisableUser(ctx context.Context, in *cognitoidentityprovider.AdminDisableUserInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminDisableUserOutput, error)
	CreateIdentityProvider(ctx context.Context, in *cognitoidentityprovider.CreateIdentityProviderInput, optFns ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.CreateIdentityProviderOutput, error)
}

// Client is the production Cognito Admin, backed by the AWS SDK.
type Client struct {
	api        cognitoAPI
	userPoolID string
}

var _ Admin = (*Client)(nil)

// Credentials describes how the Cognito client authenticates to AWS. GKE has no
// AWS instance profile, so we authenticate as an IAM user (a static access key)
// whose only privilege is assuming RoleARN — the role that actually holds the
// Cognito permissions. Any field left empty falls back to the default AWS
// credential chain, so local/dev runs (with ambient creds) keep working.
type Credentials struct {
	AccessKeyID     string // IAM user access key id
	SecretAccessKey string // IAM user secret access key
	RoleARN         string // role to assume for Cognito operations (optional)
	RoleSessionName string // STS session label (optional; defaults below)
}

// NewClient builds a Client. When static IAM-user credentials are supplied it
// signs an STS AssumeRole with them and uses the returned temporary credentials
// for Cognito; this is the GKE path (no instance profile). The AssumeRole call
// is lazy — it fires on the first Cognito request and is then cached — so merely
// constructing the client makes no network call.
func NewClient(ctx context.Context, region, userPoolID string, creds Credentials) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}

	// IAM-user static credentials become the source identity for AssumeRole.
	if creds.AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, ""),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Assume the Cognito role, sourcing from whatever credentials cfg resolved
	// (the static user above, or the default chain for local runs).
	if creds.RoleARN != "" {
		sessionName := creds.RoleSessionName
		if sessionName == "" {
			sessionName = defaultRoleSessionName
		}

		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), creds.RoleARN,
			func(o *stscreds.AssumeRoleOptions) { o.RoleSessionName = sessionName })
		cfg.Credentials = aws.NewCredentialsCache(provider)
	}

	return &Client{
		api:        cognitoidentityprovider.NewFromConfig(cfg),
		userPoolID: userPoolID,
	}, nil
}

// EnsureGroup idempotently creates the group; an existing group is not an error.
func (c *Client) EnsureGroup(ctx context.Context, group string) error {
	_, err := c.api.CreateGroup(ctx, &cognitoidentityprovider.CreateGroupInput{
		GroupName:  aws.String(group),
		UserPoolId: aws.String(c.userPoolID),
	})
	if err != nil {
		var exists *types.GroupExistsException
		if errors.As(err, &exists) {
			return nil
		}

		return fmt.Errorf("create group %q: %w", group, err)
	}

	return nil
}

// AddUserToGroup adds the user to the group.
func (c *Client) AddUserToGroup(ctx context.Context, username, group string) error {
	_, err := c.api.AdminAddUserToGroup(ctx, &cognitoidentityprovider.AdminAddUserToGroupInput{
		GroupName:  aws.String(group),
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	})
	if err != nil {
		return fmt.Errorf("add user %q to group %q: %w", username, group, err)
	}

	return nil
}

// EnsureUser idempotently creates a native Cognito user (email verified) and
// returns its sub. If the user already exists, its existing sub is fetched and
// returned. Cognito emails new users an invitation with a temporary password.
func (c *Client) EnsureUser(ctx context.Context, email string) (string, error) {
	out, err := c.api.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(email),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("email_verified"), Value: aws.String("true")},
		},
		DesiredDeliveryMediums: []types.DeliveryMediumType{types.DeliveryMediumTypeEmail},
	})
	if err != nil {
		var exists *types.UsernameExistsException
		if errors.As(err, &exists) {
			return c.getUserSub(ctx, email)
		}

		return "", fmt.Errorf("create user %q: %w", email, err)
	}

	sub := subFromAttributes(out.User.Attributes)
	if sub == "" {
		return "", fmt.Errorf("created user %q has no sub attribute", email)
	}

	return sub, nil
}

// RemoveUserFromGroup removes the user from the group.
func (c *Client) RemoveUserFromGroup(ctx context.Context, username, group string) error {
	_, err := c.api.AdminRemoveUserFromGroup(ctx, &cognitoidentityprovider.AdminRemoveUserFromGroupInput{
		GroupName:  aws.String(group),
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	})
	if err != nil {
		return fmt.Errorf("remove user %q from group %q: %w", username, group, err)
	}

	return nil
}

// DisableUser disables the user so they can no longer authenticate.
func (c *Client) DisableUser(ctx context.Context, username string) error {
	_, err := c.api.AdminDisableUser(ctx, &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	})
	if err != nil {
		return fmt.Errorf("disable user %q: %w", username, err)
	}

	return nil
}

// CreateIdentityProvider provisions an OIDC identity provider in the pool.
// Cognito auto-discovers the OIDC endpoints from the issuer.
func (c *Client) CreateIdentityProvider(ctx context.Context, idp IdentityProvider) error {
	details := map[string]string{
		"client_id":                 idp.ClientID,
		"client_secret":             idp.ClientSecret,
		"oidc_issuer":               idp.OIDCIssuer,
		"authorize_scopes":          idp.Scopes,
		"attributes_request_method": "GET",
	}

	_, err := c.api.CreateIdentityProvider(ctx, &cognitoidentityprovider.CreateIdentityProviderInput{
		UserPoolId:       aws.String(c.userPoolID),
		ProviderName:     aws.String(idp.Name),
		ProviderType:     types.IdentityProviderTypeTypeOidc,
		ProviderDetails:  details,
		AttributeMapping: idp.AttributeMap,
	})
	if err != nil {
		return fmt.Errorf("create identity provider %q: %w", idp.Name, err)
	}

	return nil
}

func (c *Client) getUserSub(ctx context.Context, email string) (string, error) {
	out, err := c.api.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return "", fmt.Errorf("get user %q: %w", email, err)
	}

	sub := subFromAttributes(out.UserAttributes)
	if sub == "" {
		return "", fmt.Errorf("existing user %q has no sub attribute", email)
	}

	return sub, nil
}

func subFromAttributes(attrs []types.AttributeType) string {
	for _, a := range attrs {
		if a.Name != nil && *a.Name == "sub" && a.Value != nil {
			return *a.Value
		}
	}

	return ""
}
