package main

var verbHelp = map[string]string{
	"status": `elt project status

Show the project's own name, repo size, and a per-file-type summary:
tracked languages, active/removed key counts, chain-navigation support,
and whether a chain source has been uploaded. This is usually the first
call worth making against an unfamiliar project or server.
`,

	"track": `elt project track <FileType> --langs=Russian,German

Start tracking a file type (Localization, Dialogues, or PDA) for the
given comma-separated target languages. Safe to call again later on an
already-tracked file type to add more languages — it never removes ones
already tracked.
`,

	"sync": `elt project sync <FileType> <csv-file> [--force]

Upload a source CSV and reconcile it against the project's current
state for that file type: new keys are added, keys whose English text
changed are marked stale (never silently overwritten), and keys no
longer present are marked removed (never deleted outright). Always
commits. Refused with a 409-shaped error if the CSV has a structural
issue at error severity (e.g. a row longer than the header) unless
--force is given — a warning-severity issue (e.g. a short row, which
may just be an intentionally omitted trailing language) never blocks it.
`,

	"export": `elt project export <FileType> [--out file.csv] [--rev <commit>]

Download the compiled, ready-to-ship CSV for one file type. Its header
always carries the project's full known-language set, not just the
languages this project happens to track — an untracked language's
column is simply left empty on every row, matching the real game CSV's
fixed-column format. --rev exports the file exactly as it was at any
past commit (see 'elt project history') instead of the current state,
without touching the live project at all.
`,

	"translate": `elt project translate <FileType> <key> <lang> <text> [--source=manual|machine] [--owner=..]

Set one key's translation text directly. --source defaults to "manual"
(a human-authored translation); pass --source=machine to record it as
an unreviewed AI draft instead (the same distinction 'elt project
confirm' flips back to manual without touching the text). Blocked with
a 409 if another participant currently holds an advisory edit lock on
this exact key+language pair (see 'elt project lock') — pass --owner to
identify yourself if this connection has no authenticated user of its
own (--data-dir, or --addr with no --token).
`,

	"confirm": `elt project confirm <FileType> <key> <lang> [--owner=..]

Mark an AI-drafted translation as reviewed: flips its Source from
"machine" to "manual" without changing the translated text itself —
the "I looked at this AI draft and it's fine" action. Only meaningful
on a key whose current Source is already "machine"; same lock rules
and --owner as 'elt project translate'.
`,

	"validate": `elt project validate <FileType> --lang=Russian

Re-run validation (BBCode/tag balance, placeholder and marker
consistency against the English source, escaped-newline consistency)
over every translation in this file type+language, refreshing each
key's stored Issues list, and commits the result. Doesn't change any
translation text — only the recorded validation state.
`,

	"ai-config-set": `elt project ai-config-set --url=... --model=... [--ai-token=..] [--reasoning-effort=..] [--translate-prompt-file=f] [--glossary-prompt-file=f]

Configure the AI provider this MACHINE's own ai-translate/
glossary-discover runs use — any OpenAI-compatible /chat/completions
endpoint (a hosted API, a local Ollama instance, Ollama behind an auth
proxy). Written to a plain local file (~/.config/elt/ai.json, this
machine's own — never any project's), so it takes no --addr/--data-dir
at all: this is exactly the same per-machine config "elt gui --addr="
uses for its own thin-client AI, not project state, and never touches
whatever server ai-translate/glossary-discover later connect to,
local or remote alike. Fully replaces the stored config on every call —
a reconfigure that doesn't resupply --translate-prompt-file/
--glossary-prompt-file falls back to this build's own compiled-in
default prompt for that task, not an empty one (see
ai-config-prompt-get below for reading either back out). A {lang} token
inside a prompt file, if present, is substituted with the job's actual
target language at request time — the file itself is stored as the
literal template, not a one-off rendering.
`,

	"ai-config-status": `elt project ai-config-status

Show this machine's own currently configured AI provider: URL, default/
per-task model and sampling settings — never the bearer token itself.
Takes no --addr/--data-dir, same reasoning as ai-config-set.
`,

	"ai-config-prompt-get": `elt project ai-config-prompt-get <translate|glossary> [--defaults]

Print one task's raw system-prompt text to stdout, unescaped — pipe it
to a file, edit it, and pass it straight back via ai-config-set's
--translate-prompt-file/--glossary-prompt-file. --defaults prints this
build's compiled-in default prompt instead of whatever this machine
currently has configured — useful both before AI has ever been
configured at all, and to see what a reconfigure would fall back to for
a task you don't resupply a prompt for.
`,

	"ai-translate": `elt project ai-translate <FileType> --langs=Russian[,German,...] [--force] [--keys=K1,K2,...] [--discover-terms] [--priority] (--addr=host:port or --data-dir=dir)

Run an AI-translate pass to completion, in this CLI process itself —
connects to a project the ordinary way (a running "elt serve"/"elt api"
via --addr, or a local directory via --data-dir, exactly like every
other verb here — see connect.go's own doc for why both work identically)
but the actual AI calls always run on THIS machine, using whatever
ai-config-set configured here, never the project's own server — the
same design "elt gui --addr=" uses for its own AI work, now shared by
the CLI. Requires ai-config-set to have been run on this machine first.

Scope defaults to every key that needs work (untranslated, stale, or
currently failing validation) — add --force to widen that to every key
regardless of its current state, or pass an explicit --keys list (e.g.
the output of glossary-usage, to retranslate only what a glossary
change affects) to bypass both. Each key is queued once per language
listed in --langs — languages are NOT bundled together per key, so a
--priority run can jump in between one key's own languages, not just
between different keys' calls. --discover-terms additionally proposes
new draft glossary terms for every key actually translated (one extra
AI call per key, tracked on separate discovered/discovery_errors
counters — a discovery failure never fails the translation it rode
along with). --priority jumps this run's AI calls ahead of any other
ai-translate/glossary-discover process running against the same
machine's AI provider — it can never interrupt a call already in
flight, only get picked next once the current one returns.

Blocks until the whole run finishes, printing "done/total" progress to
stderr as it goes and the final job summary (as JSON) to stdout on
completion — there is no separate status/cancel/list verb: the run only
ever exists for this one command's own lifetime, and Ctrl+C is the
cancel button, the same as any other long-running CLI command.
`,

	"glossary-discover": `elt project glossary-discover <FileType> --lang=Russian [--keys=K1,K2,...] [--priority] (--addr=host:port or --data-dir=dir)

ai-translate's standalone counterpart: proposes new candidate glossary
terms (draft status) for every active key in this language, or just
--keys if given, running to completion in this CLI process the same way
ai-translate does — see its own doc for the full reasoning (local AI
provider, --addr/--data-dir both work, no separate status/cancel/list
verb, Ctrl+C cancels). Never auto-approves anything — review proposed
terms with glossary-list/glossary-approve afterward. A term already
known in ANY status, including a previously rejected one, is silently
not re-added, so a rejected term stays blacklisted across repeated
discovery runs. --priority — see ai-translate's own --priority doc,
same mechanism.
`,

	"glossary-list": `elt project glossary-list --lang=Russian [--status=draft|approved|rejected|all]

List this language's glossary terms. --status defaults to showing draft
+ approved together (the ordinary curation view) while hiding rejected
ones, so the permanent blacklist doesn't clutter it — pass
--status=rejected to see just the blacklist, or --status=all for
everything.
`,

	"glossary-add": `elt project glossary-add --lang=Russian --term=... [--action=keep|translate|transliterate] [--translation=...] [--inflect] [--notes=...]

Add a new glossary term for this language. Starts in draft status —
only an approved term is ever injected into an AI-translate prompt.
Rejected by AddGlossaryTerm's own case-insensitive duplicate check if a
term with the same text already exists in ANY status (including
rejected), so a blacklisted term can never be silently resuggested this
way either.
`,

	"glossary-update": `elt project glossary-update --lang=Russian --term=... [--action=..] [--translation=..] [--inflect] [--notes=..] [--owner=..]

Replace an existing term's fields in full (action/translation/inflect/
notes) — a whole-record update, not a partial patch; omitted flags
reset to their zero value, they don't preserve the term's current
value. Its status is untouched by this call — use glossary-approve/
-unapprove/-reject to change that. Blocked with a 409 if another
participant currently holds an advisory lock on this term (see
glossary-lock); --owner identifies you if this connection has no
authenticated user of its own.
`,

	"glossary-remove": `elt project glossary-remove --lang=Russian --term=... [--owner=..]

Delete a glossary term outright. Same lock/--owner rules as
glossary-update. If you mean "stop this term from ever being
suggested/used again" rather than deleting the record entirely, prefer
glossary-reject instead — that keeps it around specifically to block
AddGlossaryTerm's own duplicate check from letting it back in.
`,

	"glossary-approve": `elt project glossary-approve --lang=Russian --term=...

Mark a term approved. Only approved terms are ever injected into an
AI-translate prompt (as a separate system message per key whose English
text contains the term) — a draft has zero effect on AI output until
approved.
`,

	"glossary-unapprove": `elt project glossary-unapprove --lang=Russian --term=...

Revert a term to draft status. Also doubles as "un-reject" — the same
action regardless of which status the term is currently leaving, since
"go back to draft" means the same thing either way; there is no
separate glossary-unreject verb.
`,

	"glossary-reject": `elt project glossary-reject --lang=Russian --term=...

Blacklist a term: marks it rejected and hides it from glossary-list's
own default view (pass --status=rejected or --status=all to see it).
Deliberately not a delete — AddGlossaryTerm's existing duplicate check
still sees a rejected term and refuses to re-add it, which is what
stops a glossary-discover-start run from resuggesting something a human
already turned down. Use glossary-unapprove to reverse this.
`,

	"glossary-usage": `elt project glossary-usage --term=...

List every key, across all three file types, whose English text
contains this term as a substring — not language-scoped, since English
source text isn't itself per-language. This is the reverse index behind
"this glossary term changed, which keys does it actually affect" — feed
its output straight into ai-translate's own --keys flag to retranslate
exactly that set.
`,

	"lock": `elt project lock <FileType> <key> --lang=Russian [--owner=..]

Take (or, called again by the same owner, heartbeat-renew) an advisory
editing lock on one key+language pair. A lock is scoped to exactly that
pair, never the whole key across every language it has — two
translators working the same key in different languages never block
each other, since they're never actually touching the same
translation. Advisory only: an unlocked key can always be written to
directly (translate/confirm refuse only when locked by someone else);
this is purely opt-in coordination between whoever is actively editing
the same thing at the same moment. Renews if you already hold it, fails
with a 409 naming the current holder if someone else does.
`,

	"unlock": `elt project unlock <FileType> <key> --lang=Russian [--owner=..]

Release a lock you hold on one key+language pair. A no-op (not an
error) if you don't currently hold it — already released, expired, or
held by someone else. An unreleased lock also expires on its own after
a few minutes with no renewal, so this is a courtesy, not the only way
a lock ever goes away.
`,

	"lock-status": `elt project lock-status <FileType> <key> --lang=Russian

Check who, if anyone, currently holds the advisory lock on one
key+language pair — a read-only query, does not acquire or renew
anything.
`,

	"lock-keys": `elt project lock-keys <FileType> <key> [<key>...] --lang=Russian [--owner=..]

Lock a whole set of keys atomically for one language — e.g. every key
in a chain or a review group worked as one unit. All-or-nothing: if any
key in the set is already held by someone else, none of them are
locked, and the response names exactly which ones were contested.
`,

	"glossary-lock": `elt project glossary-lock --lang=Russian --term=... [--owner=..]

The same advisory-lock mechanism as 'elt project lock', for one
glossary term instead of a key — taken automatically by the GUI the
moment a term's edit form opens, protecting glossary-update/-remove
(but not glossary-approve/-unapprove/-reject, which the server never
lock-checks) against a concurrent conflicting edit.
`,

	"glossary-unlock": `elt project glossary-unlock --lang=Russian --term=... [--owner=..]

Release a glossary term lock you hold — same no-op-if-not-yours
semantics as 'elt project unlock'.
`,

	"split": `elt project split <FileType> --lang=Russian --n=2 [--only-needs-work=true]

Check out N disjoint batches of keys for one language, so several
participants can divide up translation work with no risk of two people
being handed the same key. Rejected with a 400 while any batch is
already open for this exact file type+language — an existing open
batch must be merged or reclaimed first; splitting a different
language, or once nothing is open for this one, works freely.
--only-needs-work (default true) restricts candidates to keys that are
untranslated, stale, or currently failing validation; pass =false to
include every active key regardless of state.
`,

	"batches": `elt project batches <FileType>

List every currently open (checked-out, unreturned) batch for this file
type — which keys/language each one covers, when it was split, and who
(if anyone) it's assigned to. The "what's still out there" visibility
this project needs since there's no shared filesystem to just look at.
`,

	"get-batch": `elt project get-batch <FileType> <batch-id> [--out file.json]

Download one open batch's content: a JSON object of key -> {English,
current draft translation}, the file a translator would actually work
from offline before uploading a completed version via 'elt project
merge'.
`,

	"merge": `elt project merge <FileType> <batch-id> <translations.json>

Upload a completed batch and apply it, closing the batch and releasing
its keys. translations.json must be a JSON object of exactly the keys
the batch was cut with, mapping each key to its translated text:
{"Key1": "text", ...} — a key present in the batch but omitted from the
file is simply left untouched, not blanked. An empty {} still closes
the batch cleanly with nothing applied. Project-wide-coordination-gated
server-side (admin/Maintainer-only once a project has users) — dissolving
a batch, even one assigned to you, isn't unilateral self-service; see
'elt project glossary-lock's sibling doc on locks for the general
distinction between advisory coordination and access control this
project draws elsewhere too.
`,

	"reclaim": `elt project reclaim <FileType> <batch-id>

Abandon an open batch without applying anything, releasing its keys for
a future split — for when a participant never returns their batch.
Same Maintainer-only gating as merge, and for the same reason: fully
dissolving a batch is a project-wide coordination decision, not
something any one translator should be able to do unilaterally to a
batch that may not even be theirs.
`,

	"history": `elt project history [--limit N]

Show the commit history — one entry per checkpoint (sync, a manual
commit, a rollback, a compact, ...), newest first. --limit caps how
many are shown (0, the default, shows all of them).
`,

	"commit": `elt project commit <message>

Record a checkpoint of whatever's currently pending — most write
operations already auto-commit on their own (sync, translate, batch
actions, ...), so this is for bundling a batch of otherwise-uncommitted
changes under one deliberate message, or simply checkpointing "nothing
new since the last commit" (a genuine no-op in that case — no empty
commit is created).
`,

	"remote-set": `elt project remote-set --url=... [--username=..] [--remote-token=..] [--push=manual|on-commit]

Configure the backup/team git remote (GitHub, GitLab, self-hosted, any
plain git-over-HTTPS host — no provider-specific code) — for pushing a
backup, or for another participant to 'elt project clone' the whole
project from. --push=on-commit auto-pushes after every commit (a push
failure there fails that commit's own response but never rolls the
local commit back); the default, manual, only pushes when you
explicitly run 'elt project push'. The token, if given, is stored the
same gitignored, credential-grade way as AI provider tokens — never
committed, never returned by remote-status.
`,

	"remote-status": `elt project remote-status

Show the configured remote's URL, username, and push mode — never the
token itself.
`,

	"push": `elt project push

Push committed history to the configured remote (see remote-set). Fails
if no remote is configured yet.
`,

	"clone": `elt project clone --url=... [--username=..] [--token=..] <dest-dir>

Bootstrap a brand-new local project directory from an existing remote
— the one project verb that takes neither --addr nor --data-dir, since
dest-dir IS the data directory being created by this call, not an
already-existing one to open. Use this to give a new participant their
own full local copy (history included) of a project someone else
already pushed a remote for.
`,

	"rollback": `elt project rollback --rev=<commit> [--message=...]

Restore a past revision as the new current project state — recorded as
a brand-new forward commit, never a destructive rewrite: the commit(s)
being rolled back FROM stay in history exactly as they were, so rolling
back to a later commit again afterward effectively undoes this
rollback. --message defaults to naming the resolved revision if
omitted.
`,

	"gc": `elt project gc

Prune and repack the project's underlying git object store to keep it
compact. Never changes any tracked content — safe to run at any time.
Combine with 'elt project compact' (below) for real disk savings after
squashing old history: compacting alone only orphans old objects, gc's
prune step is what actually reclaims the space.
`,

	"compact": `elt project compact --boundary=<commit> [--message=...]

Squash all history up to and including <boundary> into one new root
commit carrying that commit's exact tree, then replays every commit
AFTER boundary on top of it unchanged (same author/message/content —
only the parent link changes). Unlike rollback, this genuinely discards
history: everything before the new root commit is gone (until 'elt
project gc' actually prunes the now-orphaned objects, this only rewrites
the ref graph). Admin(Maintainer)-only on a multi-user server — a
translator connecting as a non-admin participant must never be able to
rewrite the shared history every other participant depends on.
--message defaults to naming the resolved boundary if omitted.
`,

	"chain-source-upload": `elt project chain-source-upload <Dialogues|PDA> <source-file>

Upload a Dialogues.ecf or PDA.yaml as this file type's persisted chain-
navigation source — admin(Maintainer)-only, since every client (CLI, TUI,
GUI) reads the exact same server-persisted source rather than each
supplying their own, which could otherwise silently drift out of sync
with whatever CSV state the project's admin keeps current via sync.
Validated by parsing once before persisting — a malformed upload is
rejected outright rather than replacing a working source. chain-roots/
chain below both 409 until this has been done at least once for the
file type.
`,

	"chain-roots": `elt project chain-roots <Dialogues|PDA>

List every chain root — dialogue names for Dialogues, top-level chapter
titles for PDA — reachable from the file type's uploaded chain source.
409s if no source has been uploaded yet (see chain-source-upload).
`,

	"chain": `elt project chain <Dialogues|PDA> <root>

List every key reachable from one chain root, in traversal order — pure
navigation (which translatable keys make up this dialogue/chapter and
in what order), not the text itself; look the returned keys up via 'elt
project keys' or 'elt project translate'. Parsed fresh from the
uploaded chain source on every call. A dynamic jump target the source
file itself can't resolve statically (e.g. a script variable used as a
next-node reference) simply ends that branch — an inherent limitation
of static analysis on these files, not a parsing bug.
`,

	"keys": `elt project keys <FileType> --lang=Russian [--status=..] [--search=..] [--offset=N] [--limit=N]

List/filter/page through a file type's active keys for one language.
--status narrows to one of untranslated|stale|needs_review|needs_work|
ok (omit for everything). --search is a case-insensitive substring
match over the key name and English text. --offset/--limit page through
a large result server-side — this is the same paginated query every
GUI/TUI key list is built on, so it stays workable against a real
~20,000-key project.
`,

	"diff": `elt project diff <FileType> --from=<rev> [--to=<rev>]

Show what changed for a file type between two revisions: added/changed/
removed keys, comparing English text (and, given --lang implicitly via
the same mechanism the GUI's own history diff view uses, translation
text can differ too depending on what actually changed). --to defaults
to the current live state, so the common case is just "what changed
since this commit."
`,

	"clear": `elt project clear <FileType> --lang=Russian <key> [<key>...]

Reset one or more translations back to fully untranslated — both the
text AND the recorded source, not just an empty string (deliberately
distinct from setting the translation text to ""). Typical use: after
reviewing a sync's diff and deciding a stale translation no longer
applies to the new English text.
`,
}
