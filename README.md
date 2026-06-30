# Memphis

<p align="center">
<img width="704" alt="memphis_github" src="https://github.com/user-attachments/assets/dcb42aa0-9ad5-4c05-810c-db8a19bda9b1" />
</p>

**memphis is a wholistic memory and context tool for AI agents.** It is a single Go binary that gives an agent two kinds of knowledge over one substrate — plain Markdown + YAML frontmatter, versioned in Git:

- **Canon** — your team's *authoritative* knowledge: requirements, decisions, designs, roadmaps, and prompts, captured as typed Markdown, validated against real standards, and enforced in CI. This is the durable system of record — **what is true.** Agents *cite* it instead of re-litigating it.
- **Reference** — *ingested* documentation (crawled sites, imported repos) rendered as a navigable Open Knowledge Format (OKF) "filing cabinet": abundant, summarized, and searchable supporting material — **how things work.**

> **The intent:** an agent's only real constraint is its context window. So the job is choosing which true, relevant tokens to put in it. memphis separates the two properties that vector databases conflate — **authority** (is this the canonical truth the team agreed to?) and **discoverability** (can the right piece be found at the right moment?) — and serves both to agents over MCP.
>
> **Memory is Canon. Context is the budgeted projection of Canon + Reference. AI lives only in the projection. The substrate is Git.**

### Philosophy

- **Authority-first.** Canon leads and is enforced; Reference supports it. At equal relevance, authoritative artifacts rank ahead of ingested docs, and every Canon result carries a verifiable citation and its lifecycle status.
- **Deterministic where it counts.** The authority path — typing, validation, relationship integrity, the gate — is a pure function of repository state with **no LLM and no network** (a build-failing test enforces this). AI is confined to the discovery/projection layer, where it can summarize and rank but never decide what is authoritative.
- **No database, no lock-in.** The source of truth is human- and agent-readable Markdown in Git. Search indexes, the relationship graph, and summaries are *derived projections* you can delete and rebuild at will.
- **Catch problems at write time.** Bad normative knowledge is rejected by a CI gate before it lands; ingested reference docs are treated permissively. Strictness matches the cost of being wrong.

### The agent loop

**Discover** (fuzzy full-text search across both tiers) → **ground** (resolve hits to the authoritative, status-checked Canon artifact, following supersedes to the current successor) → **assemble** (pack into a token budget, Canon-first, with normative requirement text preserved verbatim). The agent recalls fuzzily, then stands on truth.

### Use cases

- **Stop agents (and people) from violating decisions you already made.** Capture ADRs and requirements as Canon; the agent retrieves the *why* and cites it.
- **Make a large docs corpus usable as agent memory** without standing up a vector store — crawl or import it into a Reference bundle and serve it over MCP.
- **Enforce requirement quality in CI** — `memphis gate` checks BCP-14 / ISO 29148 / EARS conformance and relationship integrity, emitting SARIF.
- **Promote scattered docs into authoritative artifacts** as your knowledge matures, and graduate the fuzzy half to external RAG when it outgrows the filing cabinet — while Canon always stays canonical in the repo.

The Canon authority model is a faithful Go port of the **Requirements-as-Code** (rac-core) engine; the two tools interoperate through the Open Knowledge Format. For the full design, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## Why a "filing cabinet"?
Current AI memory systems can be broken down into three types:

- **Notebook**: Built for capture, drafting, and personal connection-making. 
  - Example: 
    - An Obsidian vault is a platonic notebook: a folder of markdown files, freeform linking, and portable. Its strengths are also its limits however. It has no structured retrieval as to perform a search you need to be a full text search. It has no referential integrity as you can link to a note that doesn't exist and Obsidian will not stop you. It has no concurrency safety so it does not support two writers or you end up with conflicts. It scales to any number of notes for _one person doing their own thinking_. As soon as multiple parties and writes occur, it falters.
- **Database**: Engineered for large-scale, multi-user, precision retrieval. 
  - These are commonly vector stores (like Pinecone or Milvus) and relational databases (like PostgreSQL). 
  - They can scale to millions of items, support concurrent writes, provide ACID guarantees, and produce audit logs. 
  - The cost is complex setup, operational maintenance, heavy infrastructure, opaque embeddings that can mislead, and a deployment story that does not look anywhere as simple as a "drop these files in a folder" situation.

There needs to be a middle ground that can serve most functions without the extremes of both ends.

- **filing cabinet**: A notebook with a structured layer on top. 
  - The cabinet's drawers: the file folders, the labels, the sorting rules, etc. 
    - These are not the documents themselves. They are a navigation system that makes the documents findable without reading all of them. In the vault concept, this means frontmatter conventions, summary callouts, an index file, a backlinking discipline, and an agent loop that maintains all of that. The strengths are summary-first navigation, traceable answers, and no vector infrastructure. 
  - The limits are real too:
    - At a scale of up to about 100 articles and roughly 400,000 words, any LLM's ability to navigate via summaries and index pages appears sufficient and the overhead and complexity of a full RAG stack would likely introduce more latency and retrieval noise than it removes. But past that ceiling, summary navigation starts producing noise faster than it removes it. A RAG search and retrieval system becomes less an additional burden and more of a requirement.

## The Filing Cabinet Architecture

Open Knowledge Format knowledge bundles [https://openknowledgeformat.com/what-is-okf] are designed to be a 'human and agent' readable bundle of Markdown files with YAML frontmatter. People can author it, agents can generate it, and tools can exchange it without a central registry or proprietary SDK.

OKF bundles can be extended to provide functionality as a "filing cabinet" for AI agents - a persistent, structured knowledge store that lives outside the context window. Key properties:

- **Summary-first navigation**: Each concept has a summary callout, and the index provides inline summaries so agents can decide what to read without paying full token cost
- **Bidirectional backlinks**: Concepts track both outbound links and backlinks in frontmatter, enabling graph traversal
- **Scale ceiling detection**: The `inspect` command warns when bundles exceed ~100 concepts or ~400K tokens, signaling when to consider adding a RAG implementation instead
- **Token-aware retrieval**: Tools support token budgets and compression levels to fit responses within context windows

This architecture allows AI agents to efficiently navigate large documentation sets by reading summaries first, then drilling into specific concepts as needed.

## Canon: authoritative agent memory

The filing cabinet handles *reference* knowledge — abundant docs where recall matters more than perfection. But the knowledge that actually steers an agent — *why* you chose an approach, *what* must hold — has a different lifecycle and a higher cost of being wrong. memphis holds that as **Canon**: typed, validated, enforced artifacts that live alongside Reference in the same store.

- **Five typed artifacts** — Requirement, Decision, Design, Roadmap, Prompt. Type is *inferred* from the `##` sections an artifact contains, not declared.
- **Minted identity** — every Canon artifact carries a stable opaque ID (`<repo-key>-<12-char Crockford base32>`); cross-references resolve through an alias index, so human-readable links like `ADR-002` keep working.
- **Typed relationships** — `## Related <Type>` / `## Supersedes` edges with integrity checks: broken / ambiguous / self references, edge legality, target-type range, status-consistency (a live artifact may not point at a retired one, except via supersedes), and cycle detection.
- **Standards-mapped validation** — requirement quality is checked against **BCP-14 / RFC 8174** (only uppercase MUST/SHALL/SHOULD carry normative weight), **ISO/IEC/IEEE 29148** (singular requirements), and **EARS** patterns.
- **Lifecycle + recency** — a `## Status` per type (e.g. Proposed/Accepted/Superseded), with recency derived from Git history rather than stored timestamps.
- **A blocking gate** — `memphis gate` runs validation + relationship integrity, classifies findings as blocking or advisory per a governed policy, and emits SARIF for CI.

A store with **no Canon behaves exactly like the original memphis** — adopting the authority layer is additive. For the full picture, see [ARCHITECTURE.md](./ARCHITECTURE.md).

### Authoring Canon

```bash
# Configure the store (optional; sensible defaults otherwise)
mkdir -p .okf
cat > .okf/config.yaml <<'EOF'
repository_key: OKF
canon_roots: [canon]
ticketing:
  provider: github
EOF

# Scaffold a typed artifact with a freshly minted ID
memphis new decision canon/adr-001-use-bleve.md --title "Use Bleve for search"

# Edit the sections, then check it (and the whole corpus)
memphis gate .

# Inspect the typed relationship graph and its health
memphis relationships . --validate --summary

# Promote an ingested Reference doc into a typed Canon draft
memphis promote guides/caching.md --type decision
```

## Installation

### Download Binary

Download the latest binary for your platform from the [releases page](https://github.com/chasedputnam/memphis/releases).

### Build from Source

```bash
go install github.com/chasedputnam/memphis/cmd/memphis@latest
```

Or clone and build:

```bash
git clone https://github.com/chasedputnam/memphis.git
cd memphis
make build
```

### Apple Intelligence (optional)

On macOS 26 Tahoe with Apple Silicon, memphis can summarize directly through
Apple's on-device Foundation Models. The provider is opt-in via the `applefm`
build tag and requires a one-time Swift shim compilation. See
[docs/APPLE_INTELLIGENCE.md](docs/APPLE_INTELLIGENCE.md) for the build
workflow.

## Quick Start

### 1. Crawl a Documentation Site

```bash
memphis crawl https://docs.example.com --out ./my-bundle
```

### 2. Or Import Local Markdown

```bash
memphis import ./docs --out ./my-bundle
```

#### Ex: Enable an Existing Repository

Turn any repository with scattered Markdown files into a searchable knowledge bundle:

```bash
# Import all Markdown files from a repository
memphis import ~/repo/my-project --out ~/repo/my-project/.okf --source-name "My Project"

# Filter to specific directories or patterns
memphis import ~/repo/my-project \
  --out ~/repo/my-project/.okf \
  --source-name "My Project" \
  --include "docs/**/*.md" \
  --include "**/*.mdx" \
  --include "**/README.md" \
  --exclude "node_modules/**" \
  --exclude "vendor/**"

# Add .okf to .gitignore (optional)
echo ".okf/" >> ~/repo/my-project/.gitignore

# Serve to AI agents
memphis serve ~/repo/my-project/.okf --mcp
```

This creates a `.okf` bundle inside your repository that indexes all documentation, READMEs, ADRs, and other Markdown content. AI agents can then search and navigate your project's knowledge base.

**Example: Enable a monorepo**
```bash
memphis import ~/repo/cloud-platform \
  --out ~/repo/cloud-platform/.okf \
  --source-name "Cloud Platform" \
  --include "**/docs/**/*.md" \
  --include "**/README.md" \
  --include "**/ARCHITECTURE.md" \
  --include "**/adr/**/*.md" \
  --exclude "**/test/**" \
  --exclude "**/fixtures/**"
```

**Keep the bundle updated**
```bash
# Re-import when docs change
memphis update ~/repo/my-project/.okf --force
```

### 4. Validate Your Bundle

```bash
memphis validate ./my-bundle
```

### 5. Serve via MCP

```bash
memphis serve ./my-bundle --mcp
```

### 6. Configure Your AI Client

Add to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "my-docs": {
      "command": "memphis",
      "args": ["serve", "./my-bundle", "--mcp"]
    }
  }
}
```

## Commands

### `memphis crawl <url>`

Crawl a documentation website and create an OKF bundle.

```bash
memphis crawl https://docs.example.com --out ./bundle [options]
```

Options:
- `--out, -o` - Output directory (required)
- `--max-pages` - Maximum pages to crawl (default: 100)
- `--max-depth` - Maximum crawl depth (default: 4)
- `--include` - Include patterns (glob or regex)
- `--exclude` - Exclude patterns
- `--same-origin` - Stay on same origin (default: true)
- `--respect-robots` - Respect robots.txt (default: true)
- `--concurrency` - Fetch concurrency (default: 4)
- `--force` - Overwrite output directory
- `--dry-run` - List pages without crawling

### `memphis import <path>`

Import local files into an OKF bundle.

```bash
memphis import ./docs --out ./bundle [options]
```

Options:
- `--out` - Output directory (required)
- `--source-name` - Bundle title
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--force` - Overwrite output directory
- `--summarize` - Summarization mode: `extractive` (default) or `llm`
- `--summarize-algorithm` - Extractive algorithm: `lsa` (default), `lexrank`, `textrank`, `luhn`, `edmundson`, `sumbasic`, `kl`, `reduction`, `random`
- `--language` - Stemmer language: `english` (default), `french`, `spanish`, `russian`, `swedish`, `norwegian`, `hungarian`
- `--edmundson-config` - Path to `edmundson.config` YAML (only used when `--summarize-algorithm=edmundson`)

See [Summarization](#summarization) for what each mode and algorithm does, and how to configure LLM providers.

### `memphis validate <bundle>`

Validate an OKF bundle structure and health.

```bash
memphis validate ./bundle [--json]
```

Validates:
- Index structure and frontmatter
- Concept frontmatter (required `type` field)
- Internal link integrity (broken links)
- **Filing cabinet health**: missing summary callouts, summary length, scale ceiling

Missing summaries produce warnings (not errors) for backward compatibility with older bundles.

### `memphis inspect <bundle>`

Display bundle statistics and scale metrics.

```bash
memphis inspect ./bundle
memphis inspect ./bundle --recommendations
```

Output includes:
- Concept count, link count, broken links, orphan concepts
- Type and tag distribution
- **Scale metrics**: total tokens, average tokens per concept, index ratio
- **Scale status**: healthy, warning (approaching ceiling), or exceeded

Options:
- `--recommendations` - Show RAG graduation guidance if scale ceiling is exceeded

When a bundle exceeds ~100 concepts or ~400K tokens, `inspect` warns that the filing cabinet pattern is approaching its scale ceiling. Use `--recommendations` to see guidance on adding vector search.

### `memphis serve <bundle>`

Start an MCP server for a bundle.

```bash
memphis serve ./bundle --mcp
```

Options:
- `--mcp` - Use MCP stdio transport (default: true)
- `--name` - Server name
- `--max-result-chars` - Maximum characters in tool results (default: 12000)

### `memphis update <bundle>`

Update an existing OKF bundle from its original source.

```bash
memphis update ./bundle [options]
```

The source is automatically read from the bundle's `changelog.txt` file (created during crawl or import). You can override it with the `--source` flag.

Options:
- `--source, -s` - Override source URL or path
- `--force` - Apply all changes without prompting
- `--dry-run` - Show changes without applying them
- `--max-pages` - Maximum pages to crawl, for URL sources (default: 100)
- `--max-depth` - Maximum crawl depth, for URL sources (default: 4)
- `--concurrency` - Fetch concurrency, for URL sources (default: 4)
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--summarize` - Override the summarization mode stored in the changelog
- `--summarize-algorithm` - Override the extractive algorithm stored in the changelog
- `--language` - Override the language stored in the changelog
- `--edmundson-config` - Path to `edmundson.config` YAML

When omitted, summarization flags default to whatever was last recorded in the bundle's changelog. Overrides on `update` apply for that run only and are not persisted to the changelog header.

Example workflow:
```bash
# Initial crawl
memphis crawl https://docs.example.com --out ./my-bundle

# Later, update with changes
memphis update ./my-bundle --dry-run  # Preview changes
memphis update ./my-bundle --force    # Apply all changes
memphis update ./my-bundle            # Interactive mode
```

### `memphis demo`

Run an offline demo with the bundled example.

```bash
memphis demo [--serve]
```

### Canon commands

These operate on the **authority** tier (typed artifacts under the configured `canon_roots`). They are no-ops on a pure-Reference store.

#### `memphis new <type> <path>`

Scaffold a typed Canon artifact (`requirement`, `decision`, `design`, `roadmap`, `prompt`) with a freshly minted opaque ID and the type's required + recommended sections.

```bash
memphis new decision canon/adr-001-use-bleve.md --title "Use Bleve for search"
memphis new requirement canon/req-search.md --store .
```

#### `memphis gate [store]`

Run the unified authority gate: validate every Canon artifact, check relationship integrity, and classify findings as blocking or advisory per the store's `enforcement` policy. Exits non-zero if any blocking finding exists.

```bash
memphis gate .                 # human output
memphis gate . --json          # machine-readable result
memphis gate . --sarif         # SARIF 2.1.0 for CI required-checks
```

#### `memphis relationships [store]`

Report and validate the typed relationship graph.

```bash
memphis relationships .                       # list edges
memphis relationships . --validate            # + integrity issues (fails on errors)
memphis relationships . --summary             # + coverage / orphans / broken counts
memphis relationships . --validate --json
```

#### `memphis promote <concept> --type <type>`

Promote an ingested Reference concept into a typed Canon draft — mints an ID, scaffolds the type's sections, and seeds the concept's content into the primary prose section. The draft is then validated so it is never silently treated as authoritative.

```bash
memphis promote guides/caching.md --type decision
```

#### `memphis rebuild [store]`

Regenerate all derived indexes (full-text search, relationship graph) from the Markdown source of truth. Derived indexes are caches — deleting and rebuilding them never affects the canonical files.

```bash
memphis rebuild .
```

#### `memphis export [store]`

Export the **Reference** tier for graduation to an external RAG or graph backend when it outgrows the in-repo filing cabinet. Canon stays in the repo as the source of truth and is never exported as documents.

```bash
memphis export . --documents        # Reference concepts as JSONL (for RAG)
memphis export . --graph            # relationship graph as JSON
memphis export . --documents --out refs.jsonl
```

## MCP Tools

When serving a bundle via MCP, the following tools are available to AI agents. The server is **read-only** — all mutation happens via the CLI and Git PR review.

### Canon (authority) Tools

| Tool | Description |
|------|-------------|
| `get_artifact` | Read one authoritative Canon artifact by ID, with its type, status, relationships, and citation |
| `find_decisions` | Find Canon decisions related to a topic |
| `get_related` | Typed relationships for an artifact: outgoing references, incoming references, and a bounded multi-hop neighborhood |
| `get_summary` | Summarize the Canon corpus: counts by type and lifecycle status |

### Search & Read Tools

| Tool | Description |
|------|-------------|
| `search_concepts` | Full-text search across both tiers with token budget control |
| `read_concept` | Read a specific concept's content with compression options |
| `get_neighbors` | Find related concepts via outbound links and backlinks |
| `get_context` | Authority-aware context assembly (discover → ground → assemble) within a token budget; Canon ranks first and carries citations, reference items are marked `derived` |
| `list_types` | List all concept types in the bundle |
| `list_tags` | List all tags in the bundle |
| `bundle_summary` | Get bundle statistics, scale metrics, and index content |

### Live Update Tools

| Tool | Description |
|------|-------------|
| `check_updates` | Check if the bundle source has updates available |
| `apply_updates` | Apply pending updates from the source (regenerates summaries and backlinks) |
| `bundle_health` | Check bundle health, scale ceiling, missing summaries, and source reachability |

### Utility Tools

| Tool | Description |
|------|-------------|
| `compression_stats` | View token compression statistics for this session |

### Token Budget & Compression

The search and read tools support token-aware responses to help AI agents manage context windows efficiently:

**Parameters:**
- `token_budget` - Maximum tokens for the response (estimates using cl100k_base encoding)
- `compression` - Compression level: `none`, `light`, `medium`, `aggressive`
- `detail_level` - Detail level 0-3 (0=minimal, 3=full content)

**Compression Levels:**
| Level | Effect |
|-------|--------|
| `none` | No compression, full content |
| `light` | Normalize whitespace, collapse blank lines |
| `medium` | Light + truncate to section boundaries with outline |
| `aggressive` | Medium + aggressive truncation with retrieval hints |

**Example: Budget-Aware Search**
```json
{
  "tool": "search_concepts",
  "arguments": {
    "query": "authentication",
    "token_budget": 2000,
    "compression": "medium",
    "detail_level": 2
  }
}
```

**Example: Get Context for a Topic**
```json
{
  "tool": "get_context",
  "arguments": {
    "query": "how to authenticate users",
    "token_budget": 4000,
    "compression": "light"
  }
}
```

### Live Updates

Bundles can be updated from their original source while the MCP server is running:

**Check for Updates**
```json
{
  "tool": "check_updates",
  "arguments": {
    "timeout_seconds": 30
  }
}
```

Response includes `has_changes`, `added`, `modified`, `deleted` counts.

**Apply Updates**
```json
{
  "tool": "apply_updates",
  "arguments": {
    "confirm": true
  }
}
```

Use `dry_run: true` to preview changes without applying them.

## Open Knowledge Format

OKF bundles are directories containing Markdown files with YAML frontmatter. The format implements the filing cabinet pattern with summary callouts and bidirectional backlinks.

### Concept Format

```markdown
---
type: Guide
title: Getting Started
description: Learn how to get started with the product.
tags:
  - quickstart
  - tutorial
resource: https://docs.example.com/getting-started
backlinks:
  - concepts/authentication
  - concepts/installation
---
# Getting Started

> [!summary]
> Learn how to install and configure the product in under 5 minutes.

Your content here...
```

### Required Fields

- `type` - Concept type (Guide, API Reference, Concept, etc.)

### Optional Fields

- `title` - Document title
- `description` - Brief description (max 180 chars)
- `tags` - Array of topic tags
- `resource` - Original source URL
- `timestamp` - Last modified date
- `backlinks` - Array of concepts that link to this one (auto-generated)

### Index Format

The root `index.md` provides summary-first navigation:

```markdown
---
okf_version: "0.1"
total_concepts: 47
total_tokens: 125000
generated: 2024-01-15T10:30:00Z
---
# My Documentation Bundle

## Concepts (47)

- [[getting-started]] · Guide, quickstart, tutorial
  Learn how to install and configure the product in under 5 minutes.

- [[authentication]] · Guide, security, oauth
  Configure OAuth2 authentication with support for multiple providers.
```

### Summary Callouts

Each concept should have a summary callout after the title:

```markdown
> [!summary]
> A 1-2 sentence summary (max 200 characters) for navigation.
```

Summaries are auto-generated during `crawl` and `import` by the configured summarizer (see [Summarization](#summarization)). The default is extractive LSA over the document body; LLM mode and other extractive algorithms are available via flags.

## Summarization

Every concept in a bundle gets a `> [!summary]` callout. The summary is what makes the index inline-readable and what powers most of the filing-cabinet's token efficiency. memphis supports two summarization modes, selected per bundle by `--summarize` on `import`:

| Mode | Source | When to use |
|------|--------|-------------|
| `extractive` (default) | Embedded Go port of [sumy](https://github.com/miso-belica/sumy) — no network, fully deterministic | Default for most bundles; fast, offline, reproducible |
| `llm` | External API, Apple Intelligence, or local Ollama (see provider stack below) | When you want generative single-sentence summaries that handle link-heavy or noisy documents better |

### Extractive algorithms

Selected with `--summarize-algorithm` on `import`. Defaults to `lsa`.

| Algorithm | What it does |
|-----------|--------------|
| `lsa` (default) | Latent Semantic Analysis; SVD over the term-sentence matrix. Good general purpose. |
| `lexrank` | PageRank over an IDF-modified-cosine sentence graph. Robust on technical prose. |
| `textrank` | PageRank over a shared-word sentence graph. Cheaper than LexRank, similar quality. |
| `luhn` | Significant-word clusters. Strong for cue-heavy documents (headers, definitions). |
| `edmundson` | Cue + key + title + location heuristic. Tunable via `edmundson.config`. |
| `sumbasic` | Greedy word-probability with redundancy downweighting. |
| `kl` | KL-divergence minimization between summary and document distributions. |
| `reduction` | Sum of pairwise shared-word counts. Cheap baseline. |
| `random` | Time-seeded random sentence pick. Useful as a control. |

### Languages

`--language` configures stemming for the extractive pipeline. Supported: `english` (default), `french`, `spanish`, `russian`, `swedish`, `norwegian`, `hungarian`. Stopword filtering is currently English-only; non-English bundles still get tokenization, stemming, and scoring, but lose the stopword filter.

### LLM mode

When `--summarize=llm` is passed, memphis walks a provider stack and uses the first one that is available:

1. **External OpenAI-compatible API** — used when `api_endpoint` and `api_token` are set in `llm.config`. Honors `Retry-After` on 429s and retries 5xx with exponential backoff.
2. **Platform-native on-device LLM** — Apple Intelligence on macOS 26 Tahoe + Apple Silicon (opt-in `applefm` build tag, see [docs/APPLE_INTELLIGENCE.md](docs/APPLE_INTELLIGENCE.md)); Windows Copilot Runtime stub.
3. **Local Ollama** — `http://localhost:11434` by default, model `phi3:mini`.
4. **Extractive fallback** — if none of the above are reachable, the engine falls back to the extractive summarizer so the import never fails because the LLM was offline.

Configuration lives in `llm.config` (YAML), searched in this order: `<bundle>/llm.config`, then `~/.config/memphis/llm.config`.

```yaml
# Use an external OpenAI-compatible endpoint
api_endpoint: https://api.openai.com/v1/chat/completions
api_token: sk-...
model: gpt-4o-mini

# Or point local fallback at a custom Ollama
local_endpoint: http://localhost:11434
local_model: phi3:mini

# Optional: override the prompt
prompt_template: |
  Summarize the following document in one concise sentence (max 200 characters).
  Title: {{.Title}}
  Content:
  {{.Content}}
```

Document content is intelligently truncated to a ~8000-token budget before being sent to the LLM: headings and first paragraphs are kept preferentially over body noise, with a hard token-level fallback if even the heading-only reduction overflows.

### Apple Intelligence (macOS 26 Tahoe)

On Apple Silicon Macs running macOS 26 Tahoe with Apple Intelligence enabled, memphis can call Foundation Models directly through an in-process CGo bridge — no HTTP server, no API key, no token leaves the device. This is opt-in via the `applefm` build tag so default builds keep working on Intel Macs, older macOS, Linux, and Windows. See [docs/APPLE_INTELLIGENCE.md](docs/APPLE_INTELLIGENCE.md) for the build workflow.

When the bridge is built and available, `llm` mode automatically prefers Apple Intelligence over the Ollama fallback (the external `api_endpoint`, if configured, still wins — handy for A/B comparisons against cloud models).

## Scale Ceiling & RAG Graduation

The filing cabinet pattern works well for documentation sets up to ~100 concepts or ~400K tokens. Beyond this, query cost grows non-linearly because summary navigation produces too many candidates.

**Check your bundle's scale:**
```bash
memphis inspect ./my-bundle
```

**When the ceiling is exceeded:**
```bash
memphis inspect ./my-bundle --recommendations
```

This outputs guidance on adding vector search (RAG) alongside the wiki structure:
- Use header-based chunking (not token-count chunking)
- Recommended local vector stores: DuckDB with vss extension, ChromaDB
- Keep the wiki structure for synthesis questions; use vectors for precision lookups

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all

# Run demo
make demo
```

## End-to-end pipeline

`internal/summarize/summarizer.go` dispatches on `--summarize` to one of two pipelines:

```
Raw bytes (markdown / HTML / text)
   │
   ▼
nlp.Parser.Parse(text) ─── stripMarkdownArtifacts → splitParagraphs → tokenize → Document
   │                       (frontmatter, code fences, callouts, links, bold/italic, etc.)
   ▼
            ┌────────────── mode = extractive (default) ──────────────┐
            │                                                          │
            ▼                                                          │
sumer.NewSummarizer(algo, lang) → Summarizer                          │
   │                                                                   │
   ▼                                                                   │
summarizer.Summarize(doc, sentenceCount=1) → []*nlp.Sentence          │
   │           ↑                                                       │
   │           └─ algorithm-specific scoring (LSA, LexRank, ...)       │
   ▼                                                                   │
Base.GetBestSentences ── stable sort by rating desc, then re-sort      │
   │                     selected into document order                  │
   ▼                                                                   │
ExtractiveAdapter trims to MaxSummaryLength, returns Summary{Text}     │
   │                                                                   │
   │            ┌─────────────── mode = llm ──────────────┐            │
   │            ▼                                          │            │
   │   llm.Engine.Summarize(content, title)               │            │
   │            │                                          │            │
   │            ▼                                          │            │
   │   intelligentTruncate to ~8000 tokens                │            │
   │   (headings + first paras kept preferentially)       │            │
   │            │                                          │            │
   │            ▼                                          │            │
   │   selectProvider →  api → apple → ollama             │            │
   │            │              (Apple Foundation Models    │            │
   │            │               on macOS 26 + applefm tag) │            │
   │            ▼                                          │            │
   │   provider.Generate(ctx, prompt)                     │            │
   │            │                                          │            │
   │            ▼                                          │            │
   │   if err → fall back to extractive ──────────────────┘            │
   │                                                                   │
   └────────────────────────────────┬──────────────────────────────────┘
                                    ▼
              writer.injectSummaryCallout writes
              `> [!summary]` block into the rendered .md file
```

The extractive path always requests **one** sentence because the OKF summary callout is a single line. The LLM path wraps a single-sentence prompt template; if the configured provider is unavailable or fails, the engine silently falls back to extractive so an import never fails because the LLM was offline.

## License

MIT
