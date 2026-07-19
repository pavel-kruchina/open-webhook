---
name: open-webhook
description: >-
  START HERE to set up or use open-webhook — a self-hosted webhook / HTTP-request inspector for AI
  agents and automation. Invoke when the user wants to capture, inspect, test, or debug webhooks,
  callbacks, agent tool-calls, or provider HTTP requests, or says "run / deploy / use open-webhook".
  This is the entrypoint: it decides whether you need a LOCAL instance (testing apps on this machine)
  or a PUBLIC one (capturing requests from the internet / third parties), then hands off to the setup
  and usage skills.
allowed-tools: Read, Write, Bash
---

# open-webhook — start here

open-webhook gives you a unique URL that captures every HTTP request sent to it (method, headers,
body, uploaded files) and lets you read them back over a JSON API. Use it to debug AI-agent tool
calls, callbacks, automation triggers, and provider webhooks.

**This skill is the router. Follow these steps in order.**

## 1. Check for an existing instance first

Look in your persistent memory for a saved open-webhook instance (memory named `open-webhook-instance`
or similar). If a base URL is already saved **and** reachable (`curl -fsS {BASE}/healthz` returns
`200`), skip setup and go straight to **Using it** below — do **not** ask the user to set anything up
again.

## 2. Ask what they need it for

If there is no usable saved instance, ask the user one plain, short question:

> What do you want to use open-webhook for?
> 1. **Test apps on this machine** — capture requests from software running locally.
> 2. **Capture requests from the internet** — receive webhooks from third-party services or other
>    servers (needs a public URL).
> 3. **I already have one running** — just use an existing instance (give me its URL).

## 3. Hand off to the right skill

- **Option 1 or 2 → run the `open-webhook-serve` skill** (invoke `/open-webhook-serve`, or read
  [`../open-webhook-serve/SKILL.md`](../open-webhook-serve/SKILL.md)). It installs and runs the
  service:
  - Option 1 → **local mode** (runs on this machine, gives you a `localhost` URL).
  - Option 2 → **hosted mode** (deploys to an SSH server behind the user's domain, kept running
    across reboots). For a quick throwaway public URL without a server, it can also use an ngrok
    tunnel.

  It saves the instance URL to your memory when done.
- **Option 3 →** ask for the base URL, verify `curl -fsS {BASE}/healthz`, then save it to your memory
  as `open-webhook-instance` (see the serve skill's "Save the instance URL to memory" section).

## Using it (once an instance exists)

To create webhook URLs and read captured requests from automations, use the `open-webhook-api` skill
(invoke `/open-webhook-api`, or read [`../open-webhook-api/SKILL.md`](../open-webhook-api/SKILL.md)).
It has the API cheatsheet — create a session, hand out the URL, read/download captured data — and the
Swagger link.

## The three skills

| Skill | Use it to |
|-------|-----------|
| `/open-webhook` (this one) | Decide local vs public and wire everything up |
| `/open-webhook-serve` | Install & run the service locally or on a server, keep it alive |
| `/open-webhook-api` | Use a running instance from automations via its JSON API |

## Adding these skills to Claude Code / Codex

They live in `.claude/skills/` in this repo. **Claude Code** auto-loads them when you open the repo —
just type `/open-webhook`. **Codex** reads `.agents/skills/`; mirror them once with
`mkdir -p .agents/skills && cp -r .claude/skills/* .agents/skills/`, then use `$open-webhook`.
