//go:build integration && server
// +build integration,server

package notify_test

import (
	"context"
	"log"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/notify"
)

type fakeSender struct {
	mu    sync.Mutex
	calls int
	to    []string
	body  string
}

func (f *fakeSender) Send(_ context.Context, to []string, _, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.to = to
	f.body = body

	return nil
}

func TestNotifyAllocationFailure(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Notify Co", CustomerID: "CUST-NOTIFY"}
		require.NoError(t, database.DB.Create(org).Error)

		mk := func(email, role string, deactivated bool) {
			u := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-" + email, Email: email, Role: role, Source: models.OrgUserSourceInvited}
			require.NoError(t, database.DB.Create(u).Error)
			if deactivated {
				require.NoError(t, database.DB.Model(u).Update("deactivated_at", "NOW()").Error)
			}
		}

		mk("admin1@x.com", models.OrgUserRoleCustomerAdmin, false)
		mk("admin2@x.com", models.OrgUserRoleCustomerAdmin, false)
		mk("member@x.com", models.OrgUserRoleMember, false)
		mk("ex-admin@x.com", models.OrgUserRoleCustomerAdmin, true) // deactivated

		t.Run("emails active admins only", func(t *testing.T) {
			sender := &fakeSender{}
			n := notify.NewAdminNotifier(database, sender, log.Default())

			n.NotifyAllocationFailure(context.Background(), org.ID, "no seats available")

			require.Equal(t, 1, sender.calls)
			assert.ElementsMatch(t, []string{"admin1@x.com", "admin2@x.com"}, sender.to)
			assert.Contains(t, sender.body, "no seats available")
		})

		t.Run("no admins → no send", func(t *testing.T) {
			emptyOrg := &models.Organization{Name: "Empty", CustomerID: "CUST-EMPTY-N"}
			require.NoError(t, database.DB.Create(emptyOrg).Error)

			sender := &fakeSender{}
			n := notify.NewAdminNotifier(database, sender, log.Default())
			n.NotifyAllocationFailure(context.Background(), emptyOrg.ID, "x")

			assert.Equal(t, 0, sender.calls)
		})
	})
}
