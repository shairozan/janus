import { CodeBlock } from "@/components/ui/code-block";

// How a user generates the RSA keypair whose PUBLIC half is embedded in a
// license for run-log signing. The backend (internal/signing.LoadPublicKeyFromPEM)
// requires an RSA key in PKIX/SPKI PEM — block type "PUBLIC KEY" — so the
// instructions are explicit about format to avoid a rejected paste.

const OPENSSL = `# 1. Generate an RSA private key — KEEP THIS SECRET.
#    Janus signs the user's run logs with it; it never leaves their machine.
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out janus-signing.key

# 2. Export the matching PUBLIC key — paste THIS file's contents above.
openssl pkey -in janus-signing.key -pubout -out janus-signing.pub`;

export function KeyGenInstructions() {
  return (
    <details className="group rounded-md border border-line bg-paper/60">
      <summary className="cursor-pointer list-none px-3 py-2 text-sm font-medium text-ink transition-colors hover:text-gold">
        <span className="text-gold group-open:hidden">▸ </span>
        <span className="hidden text-gold group-open:inline">▾ </span>
        How does the user generate this key?
      </summary>
      <div className="space-y-3 border-t border-line/70 px-3 py-3">
        <p className="text-sm text-ink-muted text-pretty">
          The user creates an RSA keypair. They keep the <strong>private</strong> key (Janus signs
          their run logs with it — it never leaves their machine) and give you the{" "}
          <strong>public</strong> key to embed in the license.
        </p>
        <CodeBlock label="Generate a key (OpenSSL)" value={OPENSSL} />
        <p className="text-xs text-ink-muted text-pretty">
          Paste the contents of <span className="font-mono">janus-signing.pub</span>. It must be an
          RSA key in <span className="font-mono">-----BEGIN PUBLIC KEY-----</span> (SPKI) form — not{" "}
          <span className="font-mono">BEGIN RSA PUBLIC KEY</span> (PKCS#1), an SSH key, or an
          EC/Ed25519 key.
        </p>
      </div>
    </details>
  );
}
