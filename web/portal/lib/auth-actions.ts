"use server";

import { signIn, signOut } from "@/auth";
import { ssoAuthParams } from "./api-helpers";

// Server actions for sign-in/out. signIn redirects to Cognito's hosted UI; the
// callback returns to the portal and Auth.js completes the code→token exchange.

/** Native Cognito login (hosted-UI form) — used by pre-SSO customer admins. */
export async function loginNativeAction() {
  // Land in the app after sign-in, not the public landing page (which only shows
  // a "Sign in" CTA and would bounce an authenticated user back to /login).
  await signIn("cognito", { redirectTo: "/me" });
}

/**
 * SSO login — sends the user straight to their org's IdP via the
 * identity_provider deep-link (no provider picker). providerName is the org's
 * SSOConfiguration.ProviderName, typically supplied via /login?idp=<name>.
 */
export async function loginSSOAction(providerName: string) {
  await signIn("cognito", { redirectTo: "/me" }, ssoAuthParams(providerName));
}

/** Sign out and return to the landing page. */
export async function logoutAction() {
  await signOut({ redirectTo: "/" });
}
