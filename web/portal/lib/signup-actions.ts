"use server";

import { headers } from "next/headers";
import { licenseServerBaseUrl } from "./api-helpers";

// SignupState is the useActionState value driving the /signup form. `idle` is the
// initial form, `success` swaps to the "check your email" confirmation, `error`
// surfaces a friendly inline message (details stay server-side).
export type SignupState =
  | { status: "idle" }
  | { status: "success"; email: string }
  | { status: "error"; message: string };

export const signupInitialState: SignupState = { status: "idle" };

// signupAction is the public self-service signup: it POSTs to the license-server
// (server-side, unauthenticated — a prospect has no session yet) and maps the
// response to a SignupState. It forwards the end-user IP so the server's per-IP
// rate limit sees the real client, not this BFF.
export async function signupAction(_prev: SignupState, formData: FormData): Promise<SignupState> {
  const orgName = String(formData.get("org_name") ?? "").trim();
  const adminEmail = String(formData.get("admin_email") ?? "").trim();
  const contactName = String(formData.get("contact_name") ?? "").trim();

  if (!orgName) {
    return { status: "error", message: "Please enter your organization name." };
  }
  if (!adminEmail) {
    return { status: "error", message: "Please enter an admin email address." };
  }

  const base = licenseServerBaseUrl();
  if (!base) {
    return { status: "error", message: "Signup is temporarily unavailable. Please try again later." };
  }

  let res: Response;
  try {
    res = await fetch(`${base}/api/v1/signup`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(await forwardedForHeader()),
      },
      body: JSON.stringify({
        org_name: orgName,
        admin_email: adminEmail,
        contact_name: contactName || undefined,
      }),
      cache: "no-store",
    });
  } catch {
    return { status: "error", message: "Could not reach the signup service. Please try again." };
  }

  switch (res.status) {
    case 201:
      return { status: "success", email: adminEmail };
    case 400:
      return { status: "error", message: "Please check your details and try again." };
    case 429:
      return { status: "error", message: "Too many attempts. Please wait a little while and try again." };
    default:
      return {
        status: "error",
        message: "Something went wrong creating your account. Please try again later.",
      };
  }
}

// forwardedForHeader passes the originating client IP through to the license
// server (the first hop of the inbound X-Forwarded-For, or X-Real-IP). Empty when
// neither is present.
async function forwardedForHeader(): Promise<Record<string, string>> {
  const h = await headers();
  const ip = h.get("x-forwarded-for") ?? h.get("x-real-ip") ?? "";

  return ip ? { "X-Forwarded-For": ip } : {};
}
