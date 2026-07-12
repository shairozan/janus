// Package cognitotest provides a reusable in-memory fake of cognito.Admin for
// tests in other packages (e.g. the signup orchestration in A12).
package cognitotest

import (
	"context"
	"fmt"
	"sync"

	"github.com/pharmalytica/janus/internal/license/cognito"
)

var _ cognito.Admin = (*Fake)(nil)

// Fake is an in-memory cognito.Admin. It records calls and lets tests inject
// failures.
type Fake struct {
	mu sync.Mutex

	Groups        map[string]bool     // ensured groups
	Users         map[string]string   // email → sub
	Memberships   map[string][]string // username → groups
	DisabledUsers map[string]bool     // username → disabled
	IdPs          map[string]bool     // provisioned identity providers
	NextSub       string              // sub assigned to the next created user
	FailOn        map[string]error    // method name → error to return
	CreatedUsers  []string            // emails, in order
}

// New returns an empty Fake.
func New() *Fake {
	return &Fake{
		Groups:        map[string]bool{},
		Users:         map[string]string{},
		Memberships:   map[string][]string{},
		DisabledUsers: map[string]bool{},
		IdPs:          map[string]bool{},
		FailOn:        map[string]error{},
	}
}

func (f *Fake) fail(method string) error {
	if err, ok := f.FailOn[method]; ok {
		return err
	}

	return nil
}

// EnsureGroup records the group (idempotent).
func (f *Fake) EnsureGroup(_ context.Context, group string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("EnsureGroup"); err != nil {
		return err
	}

	f.Groups[group] = true

	return nil
}

// AddUserToGroup records the membership.
func (f *Fake) AddUserToGroup(_ context.Context, username, group string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("AddUserToGroup"); err != nil {
		return err
	}

	f.Memberships[username] = append(f.Memberships[username], group)

	return nil
}

// EnsureUser idempotently records the user and returns a sub. A repeated call
// for the same email returns the existing sub without re-recording.
func (f *Fake) EnsureUser(_ context.Context, email string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("EnsureUser"); err != nil {
		return "", err
	}

	if sub, ok := f.Users[email]; ok {
		return sub, nil
	}

	sub := f.NextSub
	if sub == "" {
		sub = "sub-" + email
	}

	f.Users[email] = sub
	f.CreatedUsers = append(f.CreatedUsers, email)

	return sub, nil
}

// RemoveUserFromGroup removes a group from the user's memberships.
func (f *Fake) RemoveUserFromGroup(_ context.Context, username, group string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("RemoveUserFromGroup"); err != nil {
		return err
	}

	kept := f.Memberships[username][:0]
	for _, g := range f.Memberships[username] {
		if g != group {
			kept = append(kept, g)
		}
	}

	f.Memberships[username] = kept

	return nil
}

// DisableUser marks the user disabled.
func (f *Fake) DisableUser(_ context.Context, username string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("DisableUser"); err != nil {
		return err
	}

	f.DisabledUsers[username] = true

	return nil
}

// CreateIdentityProvider records the provisioned IdP.
func (f *Fake) CreateIdentityProvider(_ context.Context, idp cognito.IdentityProvider) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("CreateIdentityProvider"); err != nil {
		return err
	}

	f.IdPs[idp.Name] = true

	return nil
}

// MembershipsOf returns the groups a username was added to.
func (f *Fake) MembershipsOf(username string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.Memberships[username]...)
}

// String renders the fake state (debug helper).
func (f *Fake) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return fmt.Sprintf("groups=%v users=%v memberships=%v", f.Groups, f.Users, f.Memberships)
}
