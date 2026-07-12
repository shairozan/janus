package cognito

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// mockAPI is a hand-rolled mock of the cognitoAPI subset.
type mockAPI struct {
	createGroupErr error
	addUserErr     error
	createUserOut  *cognitoidentityprovider.AdminCreateUserOutput
	createUserErr  error
	getUserOut     *cognitoidentityprovider.AdminGetUserOutput
	getUserErr     error
	removeUserErr  error
	disableUserErr error
	createIdPErr   error

	lastCreateGroup *cognitoidentityprovider.CreateGroupInput
	lastAddUser     *cognitoidentityprovider.AdminAddUserToGroupInput
	lastCreateUser  *cognitoidentityprovider.AdminCreateUserInput
	lastRemoveUser  *cognitoidentityprovider.AdminRemoveUserFromGroupInput
	lastDisableUser *cognitoidentityprovider.AdminDisableUserInput
	lastCreateIdP   *cognitoidentityprovider.CreateIdentityProviderInput
}

func (m *mockAPI) CreateGroup(_ context.Context, in *cognitoidentityprovider.CreateGroupInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.CreateGroupOutput, error) {
	m.lastCreateGroup = in

	return &cognitoidentityprovider.CreateGroupOutput{}, m.createGroupErr
}

func (m *mockAPI) AdminAddUserToGroup(_ context.Context, in *cognitoidentityprovider.AdminAddUserToGroupInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminAddUserToGroupOutput, error) {
	m.lastAddUser = in

	return &cognitoidentityprovider.AdminAddUserToGroupOutput{}, m.addUserErr
}

func (m *mockAPI) AdminCreateUser(_ context.Context, in *cognitoidentityprovider.AdminCreateUserInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error) {
	m.lastCreateUser = in

	return m.createUserOut, m.createUserErr
}

func (m *mockAPI) AdminGetUser(_ context.Context, _ *cognitoidentityprovider.AdminGetUserInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminGetUserOutput, error) {
	return m.getUserOut, m.getUserErr
}

func (m *mockAPI) AdminRemoveUserFromGroup(_ context.Context, in *cognitoidentityprovider.AdminRemoveUserFromGroupInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminRemoveUserFromGroupOutput, error) {
	m.lastRemoveUser = in

	return &cognitoidentityprovider.AdminRemoveUserFromGroupOutput{}, m.removeUserErr
}

func (m *mockAPI) AdminDisableUser(_ context.Context, in *cognitoidentityprovider.AdminDisableUserInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminDisableUserOutput, error) {
	m.lastDisableUser = in

	return &cognitoidentityprovider.AdminDisableUserOutput{}, m.disableUserErr
}

func (m *mockAPI) CreateIdentityProvider(_ context.Context, in *cognitoidentityprovider.CreateIdentityProviderInput, _ ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.CreateIdentityProviderOutput, error) {
	m.lastCreateIdP = in

	return &cognitoidentityprovider.CreateIdentityProviderOutput{}, m.createIdPErr
}

func newClient(m *mockAPI) *Client {
	return &Client{api: m, userPoolID: "us-east-2_pool"}
}

func TestEnsureGroup(t *testing.T) {
	t.Run("creates group with correct input", func(t *testing.T) {
		m := &mockAPI{}
		if err := newClient(m).EnsureGroup(context.Background(), "customer_admin"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if m.lastCreateGroup == nil || aws.ToString(m.lastCreateGroup.GroupName) != "customer_admin" {
			t.Fatalf("group name not passed correctly: %+v", m.lastCreateGroup)
		}

		if aws.ToString(m.lastCreateGroup.UserPoolId) != "us-east-2_pool" {
			t.Fatalf("user pool id not passed: %+v", m.lastCreateGroup)
		}
	})

	t.Run("existing group is idempotent (no error)", func(t *testing.T) {
		m := &mockAPI{createGroupErr: &types.GroupExistsException{}}
		if err := newClient(m).EnsureGroup(context.Background(), "customer_admin"); err != nil {
			t.Fatalf("GroupExistsException must be treated as success, got: %v", err)
		}
	})

	t.Run("other errors propagate", func(t *testing.T) {
		m := &mockAPI{createGroupErr: errors.New("access denied")}
		if err := newClient(m).EnsureGroup(context.Background(), "customer_admin"); err == nil {
			t.Fatal("expected non-GroupExists error to propagate")
		}
	})
}

func TestAddUserToGroup(t *testing.T) {
	m := &mockAPI{}
	if err := newClient(m).AddUserToGroup(context.Background(), "user@x.com", "customer_admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if aws.ToString(m.lastAddUser.Username) != "user@x.com" || aws.ToString(m.lastAddUser.GroupName) != "customer_admin" {
		t.Fatalf("inputs not passed correctly: %+v", m.lastAddUser)
	}
}

func TestEnsureUser(t *testing.T) {
	t.Run("returns sub from a newly created user", func(t *testing.T) {
		m := &mockAPI{createUserOut: &cognitoidentityprovider.AdminCreateUserOutput{
			User: &types.UserType{Attributes: []types.AttributeType{
				{Name: aws.String("email"), Value: aws.String("a@b.com")},
				{Name: aws.String("sub"), Value: aws.String("sub-xyz")},
			}},
		}}

		sub, err := newClient(m).EnsureUser(context.Background(), "a@b.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sub != "sub-xyz" {
			t.Fatalf("sub = %q, want sub-xyz", sub)
		}

		if len(m.lastCreateUser.UserAttributes) != 2 {
			t.Fatalf("expected 2 user attributes, got %d", len(m.lastCreateUser.UserAttributes))
		}
	})

	t.Run("existing user → fetches sub via AdminGetUser (idempotent)", func(t *testing.T) {
		m := &mockAPI{
			createUserErr: &types.UsernameExistsException{},
			getUserOut: &cognitoidentityprovider.AdminGetUserOutput{
				UserAttributes: []types.AttributeType{
					{Name: aws.String("sub"), Value: aws.String("sub-existing")},
				},
			},
		}

		sub, err := newClient(m).EnsureUser(context.Background(), "a@b.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sub != "sub-existing" {
			t.Fatalf("sub = %q, want sub-existing", sub)
		}
	})

	t.Run("missing sub on a new user is an error", func(t *testing.T) {
		m := &mockAPI{createUserOut: &cognitoidentityprovider.AdminCreateUserOutput{
			User: &types.UserType{Attributes: []types.AttributeType{
				{Name: aws.String("email"), Value: aws.String("a@b.com")},
			}},
		}}

		if _, err := newClient(m).EnsureUser(context.Background(), "a@b.com"); err == nil {
			t.Fatal("expected error when created user has no sub")
		}
	})
}

func TestRemoveUserFromGroupAndDisable(t *testing.T) {
	m := &mockAPI{}
	c := newClient(m)

	if err := c.RemoveUserFromGroup(context.Background(), "u@x.com", "customer_admin"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if aws.ToString(m.lastRemoveUser.Username) != "u@x.com" || aws.ToString(m.lastRemoveUser.GroupName) != "customer_admin" {
		t.Fatalf("remove inputs not passed: %+v", m.lastRemoveUser)
	}

	if err := c.DisableUser(context.Background(), "u@x.com"); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if aws.ToString(m.lastDisableUser.Username) != "u@x.com" {
		t.Fatalf("disable username not passed: %+v", m.lastDisableUser)
	}
}

func TestCreateIdentityProvider(t *testing.T) {
	m := &mockAPI{}
	err := newClient(m).CreateIdentityProvider(context.Background(), IdentityProvider{
		Name: "knomix-okta", OIDCIssuer: "https://knomix.okta.com", ClientID: "cid", ClientSecret: "sec",
		Scopes: "openid email", AttributeMap: map[string]string{"email": "email"},
	})
	if err != nil {
		t.Fatalf("create idp: %v", err)
	}

	if aws.ToString(m.lastCreateIdP.ProviderName) != "knomix-okta" {
		t.Fatalf("provider name not passed: %+v", m.lastCreateIdP)
	}

	if m.lastCreateIdP.ProviderType != types.IdentityProviderTypeTypeOidc {
		t.Fatalf("provider type = %v, want OIDC", m.lastCreateIdP.ProviderType)
	}

	if m.lastCreateIdP.ProviderDetails["oidc_issuer"] != "https://knomix.okta.com" {
		t.Fatalf("oidc_issuer not passed: %+v", m.lastCreateIdP.ProviderDetails)
	}

	if m.lastCreateIdP.ProviderDetails["client_secret"] != "sec" {
		t.Fatal("client_secret not passed to provider details")
	}
}

// TestNewClientCredentialPaths verifies the GKE credential wiring: construction
// must succeed (and make NO network call — AssumeRole is lazy) for the default
// chain, static IAM-user creds, and static creds + AssumeRole. We assert the
// client is built and configured, not that AWS accepts the creds (IQ/OQ boundary).
func TestNewClientCredentialPaths(t *testing.T) {
	cases := map[string]Credentials{
		"default chain":       {},
		"static user creds":   {AccessKeyID: "AKIA_TEST", SecretAccessKey: "secret"},
		"user creds + role":   {AccessKeyID: "AKIA_TEST", SecretAccessKey: "secret", RoleARN: "arn:aws:iam::123456789012:role/janus-cognito"},
		"role, default chain": {RoleARN: "arn:aws:iam::123456789012:role/janus-cognito"},
	}

	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := NewClient(context.Background(), "us-east-2", "us-east-2_TestPool", creds)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			if c == nil || c.api == nil {
				t.Fatal("client/api not constructed")
			}

			if c.userPoolID != "us-east-2_TestPool" {
				t.Fatalf("userPoolID = %q, want us-east-2_TestPool", c.userPoolID)
			}
		})
	}
}
