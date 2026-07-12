"use client";

import { useActionState } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";
import { signupAction, signupInitialState } from "@/lib/signup-actions";

// SignupForm is the public self-service signup form. It posts to the license
// server via the signupAction server action and swaps to a "check your email"
// confirmation on success. Client component: it needs useActionState for the
// pending/error/success transitions.
export function SignupForm() {
  const [state, action, pending] = useActionState(signupAction, signupInitialState);

  if (state.status === "success") {
    return (
      <div className="mt-8 text-left">
        <Alert tone="info" role="status">
          <p className="font-medium">Check your email</p>
          <p className="mt-1 text-ink-muted">
            We&apos;ve sent an invitation to <span className="font-medium text-ink">{state.email}</span>.
            Follow the link to set your password and finish setting up your account.
          </p>
        </Alert>
      </div>
    );
  }

  return (
    <form action={action} className="mt-8 space-y-5 text-left">
      {state.status === "error" && (
        <Alert tone="danger" role="alert">
          {state.message}
        </Alert>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="org_name">Organization name</Label>
        <Input id="org_name" name="org_name" required autoComplete="organization" placeholder="Acme Labs" />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="admin_email">Admin email</Label>
        <Input
          id="admin_email"
          name="admin_email"
          type="email"
          required
          autoComplete="email"
          placeholder="you@acme.io"
        />
        <p className="text-xs text-ink-muted">
          The first administrator. We&apos;ll email an invitation to activate the account.
        </p>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="contact_name">
          Your name <span className="font-normal text-ink-muted">(optional)</span>
        </Label>
        <Input id="contact_name" name="contact_name" autoComplete="name" placeholder="Jane Roe" />
      </div>

      <Button type="submit" size="lg" className="w-full" disabled={pending} aria-busy={pending}>
        {pending ? "Creating account…" : "Create account"}
      </Button>
    </form>
  );
}
