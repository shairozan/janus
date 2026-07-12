export const meta = {
  name: 'council',
  description: 'Convene a council on a hard question: independent positions, cross-examination, then a chair synthesis with dissent and vetoes recorded',
  whenToUse: 'Judgment-heavy calls where you want real disagreement rather than agreeable mush. Two rosters: `engineering` (5 seats) for one-way doors in the code — architecture forks, risky refactors, rollout decisions; and `business` (8 seats) for strategic evaluation — what is this product really, who buys it, what kills it. The business roster models the buy side as four separate parties with unequal power (qa-csv and procurement hold vetoes), so an unresolved veto cannot be outvoted by enthusiasm. A business run costs 17 agents. Pass args as {question, roster, artifact?, readOnly?, repo?} or just a question string.',
  phases: [
    { title: 'Deliberate', detail: 'each seat forms an independent position, blind to the others' },
    { title: 'Cross-examine', detail: 'each seat rebuts and fact-checks the others' },
    { title: 'Chair', detail: 'synthesize a judgment; record dissent and any veto' },
  ],
}

// ---------------------------------------------------------------------------
// The question under council.
//   args = "should we do X?"                                          (simplest form)
//   args = { question, roster?, artifact?, readOnly?, seats?, repo? } (fuller form)
//
// roster: 'engineering' (default) | 'business'
// repo:   defaults to the canonical root. An earlier checkout lived elsewhere
//         (pharmalytica-era); same code, wrong home. Do not hardcode a path here.
// ---------------------------------------------------------------------------
// args can arrive as a real object, or — depending on how the caller encodes it —
// as a JSON *string* of that object. Handle both. Getting this wrong is silent and
// expensive: the whole JSON blob becomes the "question", `roster` reads undefined,
// and the council quietly convenes the DEFAULT roster to answer a garbled prompt.
// It looks like it is working right up until you read the transcript.
let raw = args
if (typeof raw === 'string' && raw.trim().startsWith('{')) {
  try {
    raw = JSON.parse(raw)
  } catch (e) {
    throw new Error(`council: args looked like JSON but would not parse: ${e.message}`)
  }
}

const input = typeof raw === 'string' ? { question: raw } : (raw || {})
const QUESTION = input.question
const ROSTER = input.roster || 'engineering'
const ARTIFACT = input.artifact || 'the janus repository at its current HEAD'
const READ_ONLY = input.readOnly !== false // councils are read-only unless told otherwise
const REPO = input.repo || '/home/dbreeden/Documents/development/shairozan/janus'

if (!QUESTION) {
  throw new Error('council: no question. Pass args as a question string, or {question, roster?, artifact?, readOnly?}.')
}

// ---------------------------------------------------------------------------
// ROSTERS
//
// Each seat gets a distinct lens AND a deliberately different model, so the
// council disagrees for real reasons rather than producing five paraphrases of
// one model's prior.
// ---------------------------------------------------------------------------

// The engineering roster: tuned to janus as a codebase — a Go/fyne desktop app
// under IQ/OQ validation obligations, with a Next.js portal and a CI pipeline
// whose whole purpose is testing + artifact generation.
const ENGINEERING = [
  {
    seat: 'architect',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `Design integrity and correctness. Judge the decision against the house rules in CLAUDE.md:
orthogonal architecture, nothing constructed in lower layers (factory pattern excepted), dependencies
built at the highest layer and handed down, no global variables, no init() for cobra commands, errors
never abandoned or silently handled. Ask whether this decision makes the layering cleaner or muddier
in two years, not just today. Name the specific packages and boundaries it touches.`,
  },
  {
    seat: 'implementer',
    model: 'sonnet',
    effort: 'high',
    standing: 'advisory',
    lens: `Feasibility, cost, and test strategy. What does it actually take to build and land this?
Estimate the real surface area: files touched, mage targets, the Docker dev/CI images, cross-platform
(the Windows CGO story), and how long the feedback loop becomes. Say concretely how it would be tested
and whether the existing unit/integration/validation/gui test lanes cover it. Cheap-and-shippable is a
legitimate finding; so is "this is a month, not a week."`,
  },
  {
    seat: 'red-team',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `Adversarial. Assume this decision ships and then fails. Write the post-mortem. Attack data
integrity (run logs, model files, the execution record that cohabitates with the model), failure and
partial-failure modes, grid/scheduler edge cases, concurrency, error paths that get swallowed, and
security exposure. You are not here to be balanced. Find the way this hurts us.`,
  },
  {
    seat: 'contrarian',
    model: 'sonnet',
    effort: 'high',
    standing: 'advisory',
    lens: `Argue against the obvious answer. Surface the hidden assumptions in how the question itself
was framed — a question can smuggle in its own conclusion. Is there a cheaper option nobody proposed?
A do-nothing option? An option that solves the real problem instead of the stated one? If after genuine
effort the obvious answer really is right, say so plainly and explain what would have to be true for it
to be wrong. Do not manufacture dissent.`,
  },
  {
    seat: 'validation',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `Regulatory and user consequence. Janus carries IQ/OQ validation obligations: the scope
boundary is system-call generation and output parsing, with traceability from requirement to test.
Does this decision preserve that boundary, the self-validating state, and the execution run log? Does it
keep CI's two reasons for existing intact (testing and artifact generation)? Then step out of the code:
what does this mean for a pharmacometrician who has to trust the result and, eventually, defend it?`,
  },
]

// The business roster: judges janus as a *product*, not a codebase. The seats
// are built so the optimist and the skeptic are both obligated to be specific —
// vague enthusiasm and vague doom are equally worthless.
const BUSINESS = [
  {
    seat: 'devil\'s-advocate',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `You argue the case against Janus, and you argue it in good faith — which means you must be
SPECIFIC. Vague doom is worthless. Every risk you raise must be real and evidenced, and you must state its
magnitude honestly: a risk that is genuine but tiny should be named AND labeled tiny. Do not inflate a
papercut into an existential threat to win the argument; that is how a devil's advocate becomes noise and
gets tuned out.

Attack the premise. "A cheaper Pirana" is a claim about a market, not a fact — is that market real, and does
it buy? Probe: switching costs for a validated pharmacometrics workflow; who actually signs the purchase
order versus who wants the tool; whether Certara simply cuts price or bundles Janus out of existence;
whether "no license validation, no firewall holes" is a real buying trigger or an engineer's grievance that
no budget holder feels; whether CFR 21 Part 11 compliance claimed by a small vendor is trusted by a QA
department that has never heard of you; single-maintainer / bus-factor risk as procurement sees it. Rank
your risks by expected damage, and say plainly which ones you think are actually small.`,
    research: `Go find the GRAVEYARD. Who has already tried to build a cheaper/open alternative in
pharmacometrics tooling, and what happened to them? Search for abandoned GitHub projects in this space and
read their last commits and issues. Look for evidence of how pharma/CRO software procurement actually works
(vendor qualification, supplier audits, GAMP 5 categories) and what a small vendor must survive to get
through it. Find out what Certara has historically done to competitors — acquisitions, bundling, price
moves. Find real complaints AND real defenses of Pirana from actual users (NMusers archives, forums,
conference talks). If the incumbent is genuinely disliked, that helps the champion; if users are merely
grumbling but locked in, that is your strongest argument.`,
  },
  {
    seat: 'champion',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `You make the strongest honest case FOR Janus — and "honest" is the load-bearing word. You are not
a hype man; you are the person who has to convince a skeptical investor or a skeptical head of
pharmacometrics, and who will be humiliated if a single claim collapses under scrutiny.

Find what is genuinely, defensibly good here and name it precisely: the specific facets that a competitor
cannot trivially copy, the wedge that gets a first customer, the thing this does that the incumbent
structurally cannot or will not do. Distinguish sharply between a real moat (structural, durable) and a
feature (copyable in a quarter). Ground every strength in something in the repo — the compliance work, the
validation strategy, the orchestrator abstraction, the licensing model, the run log, whatever actually
earns it. If a claimed strength does not survive your own scrutiny, drop it rather than defend it. A
champion who oversells is worse than useless, because he costs the council its trust.`,
    research: `Go find the TAILWINDS and the unmet need. Search for what pharmacometricians and modeling
groups actually complain about today: licensing friction, firewall/VPN pain, cloud migration, cost pressure
at CROs, the difficulty of validating tools for regulated work. Look for evidence that the incumbent stack
is expensive, closed, or resented — pricing data, procurement gripes, conference talks, forum threads,
migration stories. Find out whether anyone is winning by being the open/cheaper option in an adjacent
regulated-science niche, and what made them win. Also research whether IQ/OQ documentation is itself a
paid deliverable in this industry (that would be a real asset, not a feature). Bring back citable
external evidence that the pain is real — your case is only as strong as the pain you can prove.`,
  },
  {
    seat: 'market',
    model: 'sonnet',
    effort: 'high',
    standing: 'advisory',
    lens: `The competitive and commercial landscape. Situate Janus honestly in the pharmacometrics tooling
world: Certara Pirana and the wider Certara/Simcyp stack, PsN, Pharmpy/pharmpy-based tooling, nlmixr2,
Monolix/Lixoft, R-based and homegrown workflows, and whatever else actually occupies this space. Use web
search — do not rely on memory for a market you must get right.

Answer concretely: who is the buyer (pharma sponsor? CRO? academic group? single consultant?), what do they
pay today, and what would make them switch. Size the realistic market — small and real beats large and
imaginary, and a niche that pays is a fine answer. Assess whether "cost-effective replacement" is a
durable position or a race to the bottom, and whether open-source-plus-commercial-license is the right
shape. Name where you are uncertain rather than papering over it.`,
    research: `This seat is MOSTLY research; the repo barely matters to you. Map the landscape from primary
sources. Find, with URLs: what Pirana/Certara actually costs and how it is sold (per-seat? site? bundled
with a platform?); who the real players are today and whether the space has consolidated; how many
NONMEM-using organizations plausibly exist (NONMEM license counts, ACoP/PAGE attendance, pharmacometrics
group sizes, job postings mentioning NONMEM/PsN/Pirana); whether the free/open stack (PsN + Pharmpy +
R/Rstudio) has already eaten the low end, which would be devastating to a "cheaper Pirana" thesis. Check
whether Pirana is even still actively sold and supported — an abandoned incumbent is a very different
market from a defended one. Report your sizing as a range with the reasoning shown, not a single
fabricated number.`,
  },
  {
    seat: 'strategist',
    model: 'opus',
    effort: 'high',
    standing: 'advisory',
    lens: `Business model, positioning, and the road ahead. Read licensing.md, documentation/licensing.md,
documentation/marketing/, documentation/gxp/, and the portal in web/portal/ (the self-service signup and
agreements flow says something real about the intended motion).

Ask the hard shape-of-the-business questions: is the money in licenses, support, validation packages
(IQ/OQ documentation is often what sponsors actually pay for), hosting, or something else? Is the open-core
boundary drawn in the right place? What is the wedge product versus the eventual platform? What is the
single riskiest assumption the whole plan rests on, and could it be tested cheaply this quarter? Then say
what you would actually DO in the next 90 days if this were yours. Recommend, do not survey.`,
    research: `Research the BUSINESS MODELS that work in this shape of market, not just this market. Find
real examples of open-core or dual-licensed software selling into regulated life-sciences, and what they
actually charge for (support? validation packages? hosting? indemnification? the audit trail?). Look up how
small vendors get onto an approved-supplier list at a pharma sponsor or CRO, and how long that takes —
that sales cycle length may be the single most important number in this whole council. Research whether
buyers in this space will purchase from a one-person company at all, and what structures (reseller, CRO
partnership, consultancy-first) get around that. Find out what comparable niche scientific tools charge.
Cite what you find; a 90-day plan built on imagined economics is worse than no plan.`,
  },

  // ---- The buy side ---------------------------------------------------------
  // These four are NOT one person. The one who wants it cannot approve it, and
  // the two who can kill it never open it. Modeling them as a single "customer"
  // seat averages away the conflict that actually decides the purchase.
  //
  // They evaluate Janus COLD — as an unknown vendor's tool that landed on their
  // desk. They are not told it is ours. A real QA reviewer has no loyalty.
  {
    seat: 'modeler',
    model: 'sonnet',
    effort: 'high',
    standing: 'demand',
    lens: `You are a pharmacometrician at a mid-size sponsor or CRO. You run NONMEM jobs on a grid, and your
reputation rides on results that survive audit. Some vendor's tool has landed on your desk and someone has
asked whether you would use it. Speak in the first person, from that chair.

You have NO purchasing power. You cannot approve this, you cannot sign for it, and nobody asked your
permission before buying the last tool you were made to use. What you have is demand: if you want this
badly, you can create pressure; if you shrug, it dies quietly no matter how good it is. Judge it on that
axis and be honest about which it is.

Walk the actual journey — submit, watch, cancel, browse history, generate IQ/OQ — and say what it feels
like. Where does it save you real time, and where does it merely relocate your pain? What would make you
refuse: losing a workflow you depend on, having no one to call when it breaks mid-submission, having to
learn a new tool for no gain? What would make you a zealot? Be concrete about the daily texture of the
work. Do not evaluate this from above; you have to LIVE with it.`,
    research: `Do not invent this persona from imagination — go LEARN it. Research how pharmacometricians
actually work day to day: NMusers threads, PAGE/ACoP abstracts and talks, pharmacometrics blogs, PsN and
Pirana documentation and tutorials, r/pharmacometrics, public accounts of NONMEM grid workflows. Find out
what tools they really run, what their gripes really are, and what a day actually looks like. Then speak as
that person, informed by what you found, and cite where you learned it. A persona built on vibes is
worthless to this council.`,
  },
  {
    seat: 'qa-csv',
    model: 'opus',
    effort: 'high',
    standing: 'veto',
    lens: `You are the computer system validation (CSV) reviewer in Quality Assurance. You will never open
this tool. You do not care whether it is pleasant to use. You care about exactly one thing: whether the
company can defend it to an inspector.

YOU HOLD A VETO. Your "no" ends this — no amount of enthusiasm downstairs overrides it. Understand also
that your "yes" is nearly worthless: clearing validation does not make anyone buy anything. So do not
soften. Your job is to find the reason this cannot be used for regulated work, and to state it plainly.

Interrogate: the IQ/OQ package (who wrote it, who qualified the writer, is a vendor's self-issued package
even acceptable?); the audit trail (is it attributable, legible, contemporaneous, original, accurate — and
is it tamper-evident?); the CFR 21 Part 11 claims specifically, which are easy to assert and hard to
survive; the self-validating state; whether the execution record that cohabitates with the model is
trustworthy evidence or merely a log file. Ask what happens when a run fails halfway. Ask who is
accountable when the tool is wrong.

State explicitly whether you BLOCK, and if so, exactly what would have to exist for you to withdraw the
block. "It would need X" is the most useful sentence you can produce.`,
    research: `You must not answer from memory; validation expectations move. Research what a QA/CSV review
actually demands before a modeler may use a tool for regulatory work. Look up: GAMP 5 software categories
and which one a tool like this falls into (that classification determines the entire validation burden);
the FDA's Computer Software Assurance (CSA) guidance and how it changed the calculus versus traditional CSV;
CFR 21 Part 11 and Annex 11 requirements for audit trails and electronic records; ISPE/GAMP material on
supplier assessment. Find out whether a small vendor's self-issued IQ/OQ documentation is accepted in
practice or whether the sponsor must re-validate in their own environment — that single question may decide
this council. Cite URLs. Report what you could not determine.`,
  },
  {
    seat: 'budget-holder',
    model: 'sonnet',
    effort: 'high',
    standing: 'signature',
    lens: `You are the head of pharmacometrics, or the IT director who owns this line item. You pay for it.
You will never touch it.

YOU HOLD THE SIGNATURE — but only downstream of the vetoes. You can say yes; you cannot overrule QA or
procurement, and you know better than to try. What you actually control is whether this is worth the
disruption at all.

Ask the questions a budget holder asks: what am I paying Certara today, and is that a renewal I could
credibly threaten or walk away from? What is the switching cost in people's time, in revalidation, in
retraining, in the six months where half the group is on each tool? What is my exposure if I adopt a tool
with no vendor behind it and it stops being maintained — do I now own a Go codebase nobody on my team can
read? Is the saving large enough to be worth ANY of this, or is it a rounding error against one FTE?

A cheaper tool that saves me 3% of a budget line and costs me a quarter of organizational churn is a bad
trade, and I will say so.`,
    research: `Research the money, and do not guess at it. Find what the incumbent stack actually costs and
how it is sold (per-seat? site licence? bundled into a platform deal?) — and if the pricing is opaque, say
so, because opacity is itself a finding about how this market sells. Find what switching between scientific
toolchains actually costs organizations in practice: revalidation effort, retraining, parallel-running
periods. Look for real accounts of migrations in regulated science. Find out what happens to organizations
that adopted an unmaintained open tool for regulated work. Cite URLs; a fabricated price poisons this
council.`,
  },
  {
    seat: 'procurement',
    model: 'sonnet',
    effort: 'high',
    standing: 'veto',
    lens: `You are vendor qualification / procurement. You do not care what this software does. You have not
read its feature list and you will not. You care whether this SUPPLIER can be onboarded at all.

YOU HOLD A VETO, and you exercise it routinely without apology. Your "yes" means only that a purchase order
is now legally possible — it is not an endorsement and it does not make anyone want the thing.

Run the gate: Can this entity survive a supplier audit? What is its financial viability — can it show
accounts, or is it one person? Who carries professional indemnity and liability insurance? Who indemnifies
us if this tool contributes to a bad submission? Is there source-code escrow, and if the maintainer is hit
by a bus tomorrow, what exactly do we own? Who passes our security review, and who signs our data
processing terms? What is the support SLA, and who answers the phone at 2am during a submission window?
Has this supplier ever been qualified anywhere, by anyone?

Be blunt about the bus factor. A single-maintainer supplier is not a risk to be managed, in my world — it
is usually a disqualification, and the burden is on them to show me the structure that gets around it.
State whether you BLOCK and what specifically would clear it.`,
    research: `Research how a small vendor actually gets onto an approved-supplier list at a pharma sponsor
or a CRO, and how long it takes — that sales-cycle length may be the most important number in this entire
council. Look up: supplier qualification and vendor audit requirements in GxP environments, what
documentation a supplier must produce, typical insurance and indemnification demands, whether source-code
escrow is standard. Find out whether large regulated buyers will purchase from a one-person company AT ALL,
and what structures — reseller, CRO partnership, consultancy-first, incorporation with real accounts — are
used to get around it. Find real accounts of small vendors succeeding or failing at this gate. Cite URLs.`,
  },
]

const ROSTERS = { engineering: ENGINEERING, business: BUSINESS }

const SEATS = input.seats || ROSTERS[ROSTER]

if (!SEATS) {
  throw new Error(`council: unknown roster "${ROSTER}". Known rosters: ${Object.keys(ROSTERS).join(', ')}. Or pass an explicit seats array.`)
}

// NOTE ON SIZE. The filing is a FILING, not an essay. The seats told to be
// exhaustive (champion, devil's-advocate) will otherwise write ~13KB of JSON,
// blow the structured-output limit, get truncated mid-object, fail to parse, and
// die — silently taking the two advocacy seats out of the council. Bound every
// field. Depth belongs in the quality of the argument, not its length.
const POSITION = {
  type: 'object',
  required: ['position', 'reasoning', 'evidence', 'risks', 'confidence'],
  properties: {
    position: { type: 'string', description: 'This seat\'s answer to the question, stated in one or two sentences. Take a side. HARD LIMIT: 60 words.' },
    reasoning: { type: 'string', description: 'Why, argued from this seat\'s lens. Dense, not long — make every sentence carry weight. HARD LIMIT: 400 words.' },
    evidence: {
      type: 'array',
      maxItems: 12,
      description: 'Concrete things actually inspected: file paths, docs read, commands run, sources found on the web (with URLs). Empty means you did not look. Max 12 items, one line each — cite the best, do not dump everything you read.',
      items: { type: 'string' },
    },
    risks: {
      type: 'array',
      maxItems: 8,
      description: 'What this position costs or endangers, stated honestly — and sized. Say when a real risk is nonetheless small. Max 8, ranked by expected damage, one or two sentences each.',
      items: { type: 'string' },
    },
    confidence: { type: 'string', enum: ['low', 'medium', 'high'] },
  },
}

const REBUTTAL = {
  type: 'object',
  required: ['concessions', 'objections', 'revisedRecommendation', 'confidence'],
  properties: {
    concessions: {
      type: 'array',
      maxItems: 8,
      description: 'Points from other seats that genuinely change your mind. Name the seat. If none, say so — but check honestly first. Max 8, one or two sentences each.',
      items: { type: 'string' },
    },
    objections: {
      type: 'array',
      maxItems: 8,
      description: 'Where another seat is wrong, and why. Name the seat and quote the claim you are attacking. Max 8, one or two sentences each.',
      items: { type: 'string' },
    },
    revisedRecommendation: { type: 'string', description: 'Your position after hearing the others. It is legitimate for this to be unchanged. HARD LIMIT: 250 words.' },
    confidence: { type: 'string', enum: ['low', 'medium', 'high'] },
  },
}

const VERDICT = {
  type: 'object',
  required: ['judgment', 'rationale', 'consensus', 'blockers', 'dissent', 'conditions', 'openQuestions'],
  properties: {
    judgment: { type: 'string', description: 'The council\'s answer, stated so someone could act on it tomorrow.' },
    rationale: { type: 'string', description: 'The reasoning that actually carried the room.' },
    consensus: { type: 'string', enum: ['unanimous', 'majority', 'split', 'no-consensus', 'blocked'] },
    blockers: {
      type: 'array',
      description: 'One entry for EVERY seat holding standing "veto", whether or not it blocked. A veto-holder who did NOT block must still appear, with blocking=false. If any entry has blocking=true and that block was not retired during cross-examination, consensus MUST be "blocked" — regardless of how many other seats were positive.',
      items: {
        type: 'object',
        required: ['seat', 'blocking', 'whatWouldUnblock'],
        properties: {
          seat: { type: 'string' },
          blocking: { type: 'boolean', description: 'Did this veto-holder block, and did the block survive cross-examination?' },
          whatWouldUnblock: { type: 'string', description: 'The specific thing that would have to exist for the block to be withdrawn — concrete enough to build or buy, not "improve compliance". If blocking=false, say what kept them from blocking; that is load-bearing and may be fragile.' },
        },
      },
    },
    dissent: {
      type: 'array',
      description: 'Minority positions, preserved rather than averaged away. A council that erases its dissent is worth nothing.',
      items: {
        type: 'object',
        required: ['seat', 'position', 'whyItMightBeRight'],
        properties: {
          seat: { type: 'string' },
          position: { type: 'string' },
          whyItMightBeRight: { type: 'string' },
        },
      },
    },
    conditions: {
      type: 'array',
      description: 'What must hold for the decision to stay correct — the tripwires that should make us revisit it.',
      items: { type: 'string' },
    },
    openQuestions: {
      type: 'array',
      description: 'What the council could not settle and a human must.',
      items: { type: 'string' },
    },
  },
}

const GROUNDING = {
  engineering: `- Read CLAUDE.md first. Its rules are house law, not suggestions.
- Ground every claim in something you actually looked at. An assertion with no file path behind it is an opinion, and the council has enough of those.`,

  business: `- You are judging Janus as a PRODUCT and a BUSINESS, not as a codebase. Code quality matters here only where it becomes a commercial fact (it ships or it doesn't; it passes audit or it doesn't; one person can maintain it or they can't).
- Start from what the project says about itself: README.md, licensing.md, documentation/licensing.md, documentation/marketing/, documentation/gxp/, documentation/features/, documentation/design/, CHANGELOG.md, the roadmap and milestones in git history, and the portal in web/portal/. Then judge it.

## YOU MUST RESEARCH. This is not optional.

The repository cannot answer a business question. It has no idea what Certara charges, what the FDA
currently expects, whether anyone has tried this before and failed, or what a pharmacometrician complains
about in public. **The answers to this council's question mostly live OUTSIDE this repo.** A seat that only
read the codebase has not done its job and will be discounted by the chair.

- **Use WebSearch and WebFetch.** If they are not already in your tool list, load them first:
  ToolSearch with query \`select:WebSearch,WebFetch\`.
- Do NOT answer from memory on any fact about the outside world — pricing, market size, competitors,
  regulatory expectations, who acquired whom, what is current. Your memory of this market is stale and
  possibly wrong. **Look it up.**
- Go to primary sources where you can: vendor pricing and product pages, FDA/EMA guidance documents,
  ISPE/GAMP material, conference programs (ACoP, PAGE, PAGANZ), the NMusers mailing list, r/pharmacometrics
  and adjacent forums, GitHub activity on competing tools, job postings (they reveal what shops actually
  run), pharma/CRO procurement notes, LinkedIn and company sites.
- Search several ways, not once. If your first query returns little, that is a result about your *query*,
  not about the world. Reformulate and try again before concluding "no information exists."
- **Cite URLs for every external claim.** An external claim with no URL will be treated as a guess.
- Report what you could NOT find, and say what it would take to find it. "I could not determine Pirana's
  current list price from public sources" is a genuine and useful finding — a market whose prices are
  opaque tells you something about how it sells. Do not fill a gap with a plausible-sounding number;
  a fabricated figure poisons the whole council.
- Distinguish clearly, always, between: (a) what you read in the repo, (b) what you found on the web and
  can cite, and (c) what you are inferring. Label your inferences AS inferences.

## Other rules

- Ground every claim in something you actually read. Cite the file or the URL. An unevidenced assertion is a vibe, and the council has enough of those.
- Note honestly where the repository is silent. The absence of a stated buyer, price, or go-to-market is itself a finding — but distinguish "the plan doesn't exist" from "the plan exists in the founder's head and isn't written down." You cannot see inside his head; say which one you're claiming and how confident you are.`,
}

const brief = `## The question before the council

${QUESTION}

## Under review

${ARTIFACT}

## Ground rules

- Repository: ${REPO} — "Janus", a Go + fyne.io desktop tool for managing NONMEM jobs on compute grids (SLURM/SGE/TORQUE), positioned as a cost-effective replacement for Certara Pirana, with CFR 21 Part 11 compliance and IQ/OQ validation as first-class concerns. There is also a Next.js customer portal in web/portal/.
${GROUNDING[ROSTER] || GROUNDING.engineering}
- ${READ_ONLY
    ? 'READ-ONLY. Inspect all you like. Do not edit, stage, commit, or push anything.'
    : 'You may modify files, but only within your own worktree. Do not commit or push.'}
- Take a side. A seat that hedges has abstained.
- Be specific enough to be WRONG. A claim vague enough that it cannot be falsified is not a contribution.
- **Your filing is a FILING, not an essay.** Research as deeply as you like — then file COMPACTLY. Respect
  every field limit in the output schema. An over-long answer does not get truncated politely; it fails to
  parse and your seat is dropped from the council entirely, which means you argued for nothing. Cite your
  best evidence, not all of it. Density is the discipline; length is the failure.`

// Name the roster and the seats out loud. The failure this guards against is a
// silent fallback to the default roster, which is invisible until you read a
// transcript and notice the wrong people are in the room.
log(`Convening the "${ROSTER}" roster — ${SEATS.length} seats: ${SEATS.map((s) => s.seat).join(', ')}`)
log(`Question: ${QUESTION}`)

// --- Round 1: independent positions -----------------------------------------
// Deliberately parallel and blind: no seat sees another's view yet, so we get
// real priors instead of an echo of whoever spoke first.
phase('Deliberate')

const filePosition = (s) =>
  agent(
    `${brief}

## Your seat: ${s.seat}

${s.lens}
${s.research ? `
## Your research agenda

${s.research}
` : ''}
Investigate, form your position, and return it. You are one of ${SEATS.length} council members working
independently; the others hold different lenses and you will see their positions in the next round.

Do the work before you form the view. A seat that reasons from its priors and then decorates the result
with a citation or two is the exact failure this council exists to prevent.`,
    { label: `seat:${s.seat}`, phase: 'Deliberate', model: s.model, effort: s.effort, schema: POSITION },
  ).then((p) => (p ? { ...s, ...p } : null))

const positions = (await parallel(SEATS.map((s) => () => filePosition(s)))).filter(Boolean)

// An empty chair is not a smaller council — it is a rigged one. If the champion
// dies, nobody argues FOR the thing and the verdict is negative by construction;
// if the devil's advocate dies, nobody argues against it. A seat can be lost to a
// transient API error or an over-long filing that failed to parse, and the old
// `positions.length < 3` guard happily waved that through. Retry the empty chairs,
// then refuse to convene rather than deliver a confident answer from a rigged room.
const seatedNow = () => new Set(positions.map((p) => p.seat))

let absent = SEATS.filter((s) => !seatedNow().has(s.seat))
if (absent.length) {
  log(`⚠ ${absent.length} seat(s) filed nothing: ${absent.map((s) => s.seat).join(', ')}. Retrying — a missing seat is a rigged council, not a smaller one.`)
  const retried = (await parallel(absent.map((s) => () => filePosition(s)))).filter(Boolean)
  positions.push(...retried)
}

absent = SEATS.filter((s) => !seatedNow().has(s.seat))
if (absent.length) {
  throw new Error(
    `council: ${absent.map((s) => s.seat).join(', ')} never filed a position, even after a retry. ` +
    `Refusing to convene a council with an empty chair — the missing seat's argument would simply go unmade, ` +
    `and the chair would synthesize a confident verdict from a record with a hole in it. ` +
    `Check the agent transcripts: an over-long filing that fails to parse is the usual cause.`,
  )
}

log(`All ${positions.length} seats filed. Cross-examining.`)

// --- Round 2: cross-examination ---------------------------------------------
// A barrier is genuinely correct here: every seat must read every other seat's
// position before it can rebut. This is the step that makes it a council rather
// than five parallel monologues.
phase('Cross-examine')
// Standing is printed into the record the chair actually reads. Asserting it only
// in the chair's prompt is not enough — the asymmetry has to be visible next to
// the seat that holds it, at the moment the chair is weighing that seat.
const STANDING_GLOSS = {
  veto: 'VETO — this seat\'s "no" is dispositive; its "yes" means little',
  demand: 'DEMAND — cannot approve; its indifference kills quietly',
  signature: 'SIGNATURE — can approve, but only if no veto-holder blocks',
  advisory: 'ADVISORY — argues; holds no power over the purchase',
}

const transcript = positions
  .map((p) => `### Seat: ${p.seat} (standing: ${STANDING_GLOSS[p.standing] || p.standing}; confidence: ${p.confidence})
**Position:** ${p.position}
**Reasoning:** ${p.reasoning}
**Evidence:** ${p.evidence.join('; ') || '(none cited)'}
**Risks it accepts:** ${p.risks.join('; ') || '(none named)'}`)
  .join('\n\n')

const rebuttals = (await parallel(positions.map((p) => () =>
  agent(
    `${brief}

## Your seat: ${p.seat}

${p.lens}

## The council's opening positions

${transcript}

## Your task

You filed the **${p.seat}** position. Now read the others.

Concede what genuinely lands — an unmoved council is a broken one, and the seat that never concedes is
not being rigorous, it is being stubborn. Then attack what is wrong: name the seat, quote the claim, and
say why it fails.

**Fact-check the others.** Where a seat asserted something about the outside world without a citation — a
price, a market size, a competitor's behavior, a regulatory requirement — go verify it yourself with
WebSearch/WebFetch and report what you actually found. An uncited number from another seat is exactly the
kind of thing that quietly becomes "what the council concluded," and it is your job to catch it. If you
confirm it, say so and cite the source they should have given. If it is wrong, say what the real figure is.

Then state your position again, revised or not.`,
    { label: `rebut:${p.seat}`, phase: 'Cross-examine', model: p.model, effort: p.effort, schema: REBUTTAL },
    // standing is re-attached by hand: unlike round 1, this does not spread the
    // seat object, and REBUTTAL does not return standing. Drop this and the veto
    // goes invisible to the chair at exactly the moment it matters.
  ).then((r) => (r ? { seat: p.seat, standing: p.standing, ...r } : null)),
))).filter(Boolean)

log(`${rebuttals.length} rebuttals filed. The chair will synthesize.`)

// --- Round 3: the chair ------------------------------------------------------
phase('Chair')
const rebuttalText = rebuttals
  .map((r) => `### Seat: ${r.seat} (standing: ${STANDING_GLOSS[r.standing] || r.standing}; confidence after cross-examination: ${r.confidence})
**Conceded:** ${r.concessions.join('; ') || '(nothing)'}
**Objected:** ${r.objections.join('; ') || '(nothing)'}
**Revised position:** ${r.revisedRecommendation}`)
  .join('\n\n')

const verdict = await agent(
  `${brief}

## Opening positions

${transcript}

## Cross-examination

${rebuttalText}

## Your task: chair the council

You did not sit on this council; you are reading its record. Synthesize a judgment.

Weigh the arguments, do not count the votes — a lone well-evidenced seat beats four seats agreeing from
habit. Say plainly where the council converged and where it did not. **Preserve the dissent**: record the
minority position and the world in which it turns out to be right. Do not average the views into a mush
that no member would defend.

## Standing is not the same as persuasiveness

The seats do not have equal power, and the record above tells you which is which. Read it.

The count of positive seats is NOT the judgment. This council can and should be able to return "seven of
eight seats liked it, and the answer is still no." That is not a failure of the council; that is the
council working.

- A seat with VETO standing that blocks, and whose block was not retired during cross-examination, means the
  consensus is **blocked** — no matter how strong the champion was, no matter how many seats were positive.
  Do not average a veto into a dissent bullet. Do not let a majority outvote it. It does not work that way
  in the world you are modeling.
- A veto-holder's *approval*, by contrast, is nearly worthless as a buy signal. Passing validation or vendor
  qualification does not make anyone want the thing. Do not read it as enthusiasm.
- A DEMAND seat's enthusiasm cannot close anything. Its indifference, however, is fatal — quietly. Weigh it
  as a necessary condition, never a sufficient one.
- A SIGNATURE seat can say yes only downstream of the vetoes.

You MUST fill \`blockers\` with one entry for EVERY veto-holding seat, including those that did not block.

If a veto was RETIRED during cross-examination — the veto-holder conceded that some specific thing genuinely
satisfies them — say so explicitly and loudly in your rationale. That is the single most informative event
this council can produce, and it must not be buried as a concession bullet.

Discount any seat that argued its brief instead of the evidence. The devil's advocate is *supposed* to
attack and the champion is *supposed* to defend; that is their office, not their credibility. Weigh what
each one actually proved, and note where a seat conceded against its own brief — those concessions are the
most informative thing in the record.

State the judgment so that a human could act on it tomorrow, the conditions under which it stops being
true, and whatever the council genuinely could not settle.`,
  { label: 'chair', phase: 'Chair', model: 'opus', effort: 'high', schema: VERDICT },
)

return {
  question: QUESTION,
  seats: positions.map((p) => ({ seat: p.seat, model: p.model, standing: p.standing, opening: p.position, confidence: p.confidence })),
  rebuttals,
  verdict,
}
