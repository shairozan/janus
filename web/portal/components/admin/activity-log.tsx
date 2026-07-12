import { Badge } from "@/components/ui/badge";
import type { AuditEvent } from "@/lib/types";

// Read-only view of the org's audit_events (who did what). Append-only on the
// server — there are no controls here, only history.

function statusTone(status: number): "success" | "warning" | "danger" {
  if (status >= 500) {
    return "danger";
  }

  if (status >= 400) {
    return "warning";
  }

  return "success";
}

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }

  return d.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export function ActivityLog({ events }: { events: AuditEvent[] }) {
  if (events.length === 0) {
    return <p className="text-sm text-ink-muted">No activity yet.</p>;
  }

  return (
    <ul className="divide-y divide-line rounded-2xl border border-line bg-surface shadow-card">
      {events.map((e) => (
        <li key={e.id} className="flex items-center justify-between gap-4 px-6 py-4">
          <div className="min-w-0">
            <p className="text-sm text-ink">
              <span className="font-medium">{e.actor_label || "system"}</span>{" "}
              <span className="text-ink-muted">
                {e.action} {e.resource}
                {e.resource_id ? ` #${e.resource_id}` : ""}
              </span>
            </p>
            <p className="mt-0.5 truncate font-mono text-xs text-ink-muted">
              {e.request_method} {e.request_path}
            </p>
          </div>
          <div className="flex shrink-0 flex-col items-end gap-1">
            <Badge tone={statusTone(e.result_status)}>{e.result_status}</Badge>
            <time className="text-xs text-ink-muted" dateTime={e.occurred_at}>
              {formatWhen(e.occurred_at)}
            </time>
          </div>
        </li>
      ))}
    </ul>
  );
}
