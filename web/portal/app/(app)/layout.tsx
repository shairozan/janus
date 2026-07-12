import { auth } from "@/auth";
import { logoutAction } from "@/lib/auth-actions";
import { getProfile } from "@/lib/portal-api";
import { ROLE_ADMIN, type Profile } from "@/lib/types";
import { NavLink } from "@/components/nav-link";
import { AdminNav } from "@/components/admin-nav";

// Authenticated portal shell. Routes under app/(app)/ render inside it (the
// route group's parentheses keep "(app)" out of the URL). Middleware guards
// these paths, so a session is expected here.
export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const session = await auth();

  // Profile drives the nav (role + org). Degrade gracefully if the
  // license-server is unreachable — the page itself will surface the error.
  let profile: Profile | null = null;
  try {
    profile = await getProfile();
  } catch {
    profile = null;
  }

  const email = profile?.email ?? session?.user?.email ?? "Account";
  const isAdmin = profile?.role === ROLE_ADMIN;
  const isStaff = profile?.is_staff ?? false;
  const orgID = profile?.organization_id;

  return (
    <div className="flex flex-1 flex-col">
      <nav className="border-b border-line bg-paper/60">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6">
          <div className="flex gap-1">
            <NavLink href="/me">Profile</NavLink>
            {isAdmin && orgID != null && (
              <>
                <NavLink href={`/orgs/${orgID}/members`}>Members</NavLink>
                <NavLink href={`/orgs/${orgID}/requests`}>Requests</NavLink>
                <NavLink href={`/orgs/${orgID}/billing`}>Billing</NavLink>
                <NavLink href={`/orgs/${orgID}/rules`}>Rules</NavLink>
                <NavLink href={`/orgs/${orgID}/seats`}>Seats</NavLink>
                <NavLink href={`/orgs/${orgID}/sso`}>SSO</NavLink>
                <NavLink href={`/orgs/${orgID}/activity`}>Activity</NavLink>
              </>
            )}
            {isStaff && <AdminNav />}
          </div>
          <div className="flex items-center gap-4 py-2">
            <span className="text-xs text-ink-muted">{email}</span>
            <form action={logoutAction}>
              <button className="text-xs font-medium text-ink-muted transition-colors hover:text-ink">
                Sign out
              </button>
            </form>
          </div>
        </div>
      </nav>

      <main className="mx-auto w-full max-w-5xl flex-1 px-6 py-8">{children}</main>
    </div>
  );
}
