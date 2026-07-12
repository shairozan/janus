# Janus Frontend Design Standards

This document records the design decisions settled on for the Janus management
portal (`web/portal/`) — the aesthetic direction, the design tokens, the type
system, and the conventions every screen follows. It is the **basis for future
frontend work**: when building or reviewing UI, start here, reuse what's defined,
and extend it deliberately rather than inventing parallel styles.

> **Process note.** Frontend work uses the `frontend-design` plugin/skill for a
> design-quality pass (see `CLAUDE.md`). This document is the project-specific
> brief that pass should honor — the plugin provides the craft, this document
> provides the constraints and the "why."

---

## 1. Aesthetic direction — neoclassical editorial

The brand is **Janus**, the Roman god of doorways, thresholds, and duality (the
two faces; the logo's arch and flanking columns). The portal governs access and
licensing — thresholds, literally. The design leans into that:

- **Tone:** established, trustworthy, classical — a "licensing authority," not a
  generic SaaS dashboard. Refined and minimal, never busy.
- **The throughline:** doorway/threshold/duality. Small classical motifs (an
  arch, a keystone, columns, inscriptional type) are on-brand; use them sparingly
  and tastefully, never kitsch.
- **What we avoid:** "AI-slop" defaults — Inter/Roboto/system fonts, purple
  gradients on white, evenly-distributed timid palettes, cookie-cutter card grids
  with no point of view.

The rule of thumb: **cream paper, deep navy ink, gold-leaf accent.** Dominant
calm surfaces with sharp, sparing gold highlights outperform busy color.

---

## 2. Color — design tokens

Colors are defined **once** as CSS variables in `app/globals.css` and exposed as
Tailwind utilities in `tailwind.config.ts`. Always use the tokens (e.g.
`bg-paper`, `text-ink`, `text-gold`) — never raw hex in components. Re-theming
the whole portal is then a one-file change.

| Token              | Value     | Use                                              |
| ------------------ | --------- | ------------------------------------------------ |
| `--paper`          | `#f6f2e9` | page background (warm cream)                      |
| `--surface`        | `#fbf9f3` | raised cards / panels                            |
| `--ink`            | `#16314f` | primary text (deep navy)                         |
| `--ink-muted`      | `#5b6573` | secondary text (slate)                           |
| `--brand`          | `#1e3a5f` | the logo navy (brand fills, wordmark)            |
| `--gold`           | `#b9912f` | accent — rules, focus rings, small highlights    |
| `--gold-soft`      | `#d8c79a` | faint gold washes / hairlines                    |
| `--line`           | `#e6ddca` | warm hairline borders                            |

Tailwind utilities: `paper`, `surface`, `brand`, `line`, `ink` (+ `ink-muted`),
`gold` (+ `gold-soft`). Example: `border-line bg-surface text-ink shadow-card`.

**Gold is an accent, not a fill.** Use it for thin rules, focus rings, hover
washes (`hover:bg-gold/10`), and the occasional motif — not large surfaces. The
focus ring is gold by default (`:focus-visible` in `globals.css`).

---

## 3. Typography

A distinctive pairing, loaded via `next/font` in `app/layout.tsx` and exposed as
`font-display` / `font-sans`:

- **Display — Fraunces** (`font-display`): a soft, high-character old-style serif.
  Headings, the wordmark, hero text. Gives the classical/inscriptional feel.
  Italic is available and used for secondary emphasis (e.g. the landing heading's
  "_Management Portal_").
- **Body / UI — Hanken Grotesk** (`font-sans`): a warm humanist sans. All body
  copy, form labels, table data, buttons. Chosen for legibility in the
  data-dense admin screens (B3+).

Do **not** introduce Inter, Roboto, Arial, or system fonts. If a third face is
ever needed (e.g. a mono for keys/JTIs), add it as a token, don't inline it.

Scale: lean on Tailwind's type scale; headings in `font-display`, everything else
in `font-sans` (the body default). Prefer `text-balance` on headings and
`text-pretty` on paragraphs.

---

## 4. Spacing, surfaces, motion

- **Spacing:** generous whitespace; let content breathe. Use Tailwind's spacing
  scale consistently (multiples of 4). Avoid cramped, dense layouts unless a data
  table genuinely calls for controlled density.
- **Surfaces:** content sits on `bg-surface` panels with `border-line` hairlines
  and `shadow-card` (a soft, warm-tinted elevation — never harsh black shadows).
  Rounded corners (`rounded-xl`/`rounded-2xl` for panels, `rounded-md` for
  controls).
- **Motion:** restrained. High-impact, low-frequency — a tasteful hover/focus
  transition (`transition-colors`), the occasional staggered page-load reveal.
  No gratuitous animation. CSS-first; reach for a motion library only when a
  screen genuinely benefits.

---

## 5. Components

- **Primitives live in `components/ui/`** (shadcn/ui style: a `cva` recipe + the
  `cn()` helper from `lib/utils.ts`). The first is `Button` (variants:
  `primary` navy fill, `outline` gold hairline, `ghost` text-only). **Reuse and
  extend these**; don't hand-roll one-off button styles in screens.
- Compose screens from primitives + tokens. A new repeated pattern (input, card,
  table, badge) becomes a `components/ui/` primitive, not copy-paste.

---

## 6. Accessibility & quality bar

- Semantic HTML and ARIA roles; every interactive element is keyboard-reachable
  with a visible (gold) focus ring.
- Decorative elements (motifs, rules) are `aria-hidden`.
- Color is never the only signal; maintain sufficient contrast (navy-on-cream is
  strong; check muted text).
- Every change keeps `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `pnpm build`
  green. Component behavior is covered by Vitest + React Testing Library; critical
  journeys by the hermetic Playwright suite.

---

## 7. Fonts & build (decided)

Fonts are loaded with `next/font/google`, which fetches the woff2 files **at
build time** and **self-hosts them in the build output**. We accept the
build-time fetch from `fonts.googleapis.com`:

- The fetch happens once, during `next build`. The fonts are then **baked into
  the image** — the resulting container is self-contained and serves no requests
  to Google at runtime, so it remains hermetically sealed in production.
- The portal Docker build (Part C) therefore needs network during the build
  step only. That's an accepted, normal cost; no need to vendor font binaries.

If a future constraint requires a fully offline build, switching to
`next/font/local` is a drop-in change — nothing else in this document is
affected.

---

## Reference

- Tokens: `web/portal/app/globals.css`, `web/portal/tailwind.config.ts`
- Type + header: `web/portal/app/layout.tsx`
- Primitives: `web/portal/components/ui/`
- Landing (worked example of the direction): `web/portal/app/page.tsx`
