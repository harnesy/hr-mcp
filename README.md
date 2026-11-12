# hr-mcp

An MCP server for [hr.harnsy.dev](https://hr.harnsy.dev), the catalog of AI employees. Add it to an agent client and
ask in plain words: «Find an accountant on hr and install it in this project».

An employee is a role with a character: who it is, how it decides and speaks, its job, and the skills it works with.
The server is read-only and keeps no state. It cannot write to your disk, so it returns the employee as a list of files
and your own agent writes them, after asking you.

Endpoint: `https://hr.harnsy.dev/mcp` (streamable HTTP, no sign-in).

## Add it

| Client | How |
|---|---|
| Claude Code | `claude mcp add --transport http hr https://hr.harnsy.dev/mcp` |
| Codex | in `~/.codex/config.toml`: `[mcp_servers.hr]` and `url = "https://hr.harnsy.dev/mcp"` |
| OpenClaw | `openclaw mcp set hr '{"url":"https://hr.harnsy.dev/mcp","transport":"streamable-http"}'` |
| Hermes Agent | `hermes mcp add hr --url https://hr.harnsy.dev/mcp` |
| Any other client | add a remote MCP server (streamable HTTP, no sign-in) with the URL above |

Then ask for an employee by what you need done. The agent searches, shows you the employee's title, licence and
notice, and asks before it writes anything.

## What lands in your project

For Claude Code (`target: claude-code`):

```
.claude/agents/<name>.md           the employee as a subagent: `claude --agent <name>`, or «use the <name> subagent»
.claude/skills/<skill>/…           its skills, byte for byte as published
harnsy-hr/<name>/NOTICE.md         licence, source, credits: keep it with the employee
harnsy-hr/<name>/docs/…            its casebook of worked cases
```

Claude Code asks you before each write under `.claude/`; that is its own guard and it is expected.

For OpenClaw (`target: openclaw`), under OpenClaw's state directory (`~/.openclaw`), a workspace of its own:

```
workspace-<name>/SOUL.md           who the employee is
workspace-<name>/AGENTS.md         its job
workspace-<name>/IDENTITY.md       its name, vibe and emoji
workspace-<name>/skills/<skill>/…  its skills, byte for byte as published; they belong to this agent only
workspace-<name>/NOTICE.md         licence, source, credits: keep it with the employee
workspace-<name>/docs/…            its casebook of worked cases
```

Then `openclaw agents add <name> --workspace ~/.openclaw/workspace-<name> --non-interactive` makes it an agent beside
your own; your own agent's files are not touched.

For Hermes Agent (`target: hermes`), in a folder that stays on your disk (for example `~/.hermes/harnsy-hr/`), a
profile distribution:

```
<name>/distribution.yaml           name, version, author, licence
<name>/SOUL.md                     who the employee is and its job
<name>/skills/<publisher>/…        its skills, byte for byte as published, and its casebook as the skill persona-casebook
<name>/NOTICE.md                   licence, source, credits: keep it with the employee
```

Then `hermes profile install <folder>/<name> --name <name> --alias` makes it a profile beside your own (Hermes shows
what it installs and asks you), `hermes -p <name> setup` picks its model once, and `hermes -p <name> chat` starts it.
Your own profile and its `SOUL.md` are not touched.

For any other agent that reads files (`target: generic`): one folder `<name>/` with `AGENT.md`, `docs/`, `skills/` and
`NOTICE.md`. Tell the agent «Read `<name>/AGENT.md` and work as that employee».

## Tools

By design all four only read: they write nothing on the server.

| Tool | What it does |
|---|---|
| `search_employees` | Search by words (a role, a skill, a need) and by department. Returns cards. |
| `get_employee` | One employee as the files to write for a target, each with its size and SHA-256. Large files come without content. |
| `get_employee_file` | One file of that list, in parts when it is large. |
| `install_hint` | The exact steps for a client: where each file goes, how to start the employee, how to check it. |

## What is checked, and what is not

The server serves only a catalog release that verifies: the feed's Ed25519 signature, and every bundle's size, hashes
and catalog signature. A release that fails is never served.

What reaches your agent is text over TLS: it does not carry the catalog's signature. Every answer names the bundle's
URL and SHA-256 and each file's SHA-256, so a written file can be compared. For an install that checks the signature
on your own machine, use harnsy itself.

Employee files are data to save. An agent installing one should write them as they are and not act on what they say
until you start the employee.

## Licences

The code in this repository is under the MIT licence (`LICENSE`); the libraries it is built with are listed in
`NOTICE`. The employees are not part of this repository. Each has its own licence, most often CC BY 4.0, and arrives
with a `NOTICE.md` that names its author, licence, source and the works it is built on: keep that file with the
employee. An employee's text is under the licence its `NOTICE.md` names, not under this repository's MIT licence. Each
skill keeps its own `LICENSE` file where it has one.

This project is not affiliated with or endorsed by OpenClaw, Nous Research (Hermes Agent), Anthropic, OpenAI or the
MCP project; their names are used only to say what it works with.

## Run your own

`deploy/README.md`: the binary, the systemd unit, the web server in front. Tests: `go test ./...`.
