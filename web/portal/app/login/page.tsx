import Link from "next/link";
import { redirect } from "next/navigation";
import { auth } from "@/auth";
import { Button } from "@/components/ui/button";
import { loginNativeAction, loginSSOAction } from "@/lib/auth-actions";

// /login is the sign-in entry. Two paths:
//  • native — pre-SSO customer admins sign in with the Cognito hosted-UI form.
//  • SSO    — users arriving at /login?idp=<provider> (their org distributes
//             this link) go straight to their IdP via the identity_provider
//             deep-link.
export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ idp?: string }>;
}) {
  // Already signed in? Don't show the form (it would loop back through Cognito);
  // send them into the app.
  const session = await auth();
  if (session) {
    redirect("/me");
  }

  const { idp } = await searchParams;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col items-center justify-center px-6 py-16">
      <section className="w-full max-w-sm rounded-2xl border border-line bg-surface px-10 py-12 text-center shadow-card">
        <h1 className="font-display text-3xl text-ink">Sign in</h1>
        <div aria-hidden className="mx-auto mt-5 h-px w-12 bg-gold/70" />

        {idp ? (
          <>
            <p className="mt-6 text-sm leading-relaxed text-ink-muted">
              Continue to your organization&apos;s single sign-on.
            </p>
            <form action={loginSSOAction.bind(null, idp)} className="mt-8">
              <Button className="w-full" size="lg">
                Continue with {idp}
              </Button>
            </form>
            <Link href="/login" className="mt-4 inline-block text-xs text-ink-muted hover:text-ink">
              Sign in another way
            </Link>
          </>
        ) : (
          <>
            <p className="mt-6 text-sm leading-relaxed text-ink-muted">
              Sign in to manage your licenses, keys, and organization.
            </p>
            <form action={loginNativeAction} className="mt-8">
              <Button className="w-full" size="lg">
                Sign in
              </Button>
            </form>
            <p className="mt-6 text-xs text-ink-muted">
              New to Janus?{" "}
              <Link href="/signup" className="text-ink underline-offset-4 hover:underline">
                Create an account
              </Link>
            </p>
          </>
        )}
      </section>
    </main>
  );
}
