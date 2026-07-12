import Link from "next/link";
import { redirect } from "next/navigation";
import { auth } from "@/auth";
import { SignupForm } from "@/components/signup/signup-form";

// /signup is the public self-service entry point: a brand-new prospect creates
// their organization and first admin. It sits outside the (app) group (no auth
// required). An already-signed-in user has an account — send them into the app.
export default async function SignupPage() {
  const session = await auth();
  if (session) {
    redirect("/me");
  }

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col items-center justify-center px-6 py-16">
      <section className="w-full max-w-md rounded-2xl border border-line bg-surface px-10 py-12 text-center shadow-card">
        <h1 className="font-display text-3xl text-ink">Create your account</h1>
        <div aria-hidden className="mx-auto mt-5 h-px w-12 bg-gold/70" />
        <p className="mt-6 text-sm leading-relaxed text-ink-muted">
          Set up your organization to manage licenses, signing keys, and members.
        </p>

        <SignupForm />

        <p className="mt-8 text-xs text-ink-muted">
          Already have an account?{" "}
          <Link href="/login" className="text-ink underline-offset-4 hover:underline">
            Sign in
          </Link>
        </p>
      </section>
    </main>
  );
}
