"use client";

import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { LicenseRequest } from "@/lib/types";

interface LicenseSectionProps {
  license: string | null;
  requests: LicenseRequest[];
}

const statusTone: Record<string, "success" | "warning" | "danger" | "neutral"> = {
  approved: "success",
  pending: "warning",
  rejected: "danger",
  failed: "danger",
};

export function LicenseSection({ license, requests }: LicenseSectionProps) {
  const [copied, setCopied] = useState(false);

  async function download() {
    if (!license) return;
    const blob = new Blob([license], { type: "application/jwt" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "janus-license.jwt";
    a.click();
    URL.revokeObjectURL(url);
    setCopied(true);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>License</CardTitle>
        <CardDescription>Your current Janus license and request history.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {license ? (
          <div className="flex items-center justify-between rounded-lg border border-line bg-white px-4 py-3">
            <span className="text-sm text-ink">Your license is ready.</span>
            <Button size="sm" onClick={download}>
              {copied ? "Downloaded" : "Download license"}
            </Button>
          </div>
        ) : (
          <p className="text-sm text-ink-muted">
            No active license yet. Once a request is approved, your license will be available here.
          </p>
        )}

        {requests.length > 0 && (
          <ul className="divide-y divide-line border-t border-line">
            {requests.map((r) => (
              <li key={r.id} className="flex items-center justify-between py-3 text-sm">
                <span className="text-ink-muted">
                  Request #{r.id}
                  {r.reason ? ` — ${r.reason}` : ""}
                </span>
                <Badge tone={statusTone[r.status] ?? "neutral"}>{r.status}</Badge>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
