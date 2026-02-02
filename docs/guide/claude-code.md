---
title: Claude Code Skill
---

# Claude Code Skill

Tap provides an [Agent Skill](https://docs.anthropic.com/en/docs/claude-code/skills) for Claude Code that teaches the AI assistant best practices for creating Tap presentations. When installed, Claude Code understands Tap's markdown syntax, layouts, live code execution, and all other features.

## Installation

Install the Tap skill using the skills CLI:

```bash
npx @anthropic-ai/skills add minicodemonkey/tap
```

The skill is installed to `~/.claude/skills/` and automatically loaded in future Claude Code sessions.

## What's Included

The skill teaches Claude Code about:

- **Slide syntax** - Markdown structure, slide separators, frontmatter
- **Layouts** - All 11 layouts and their slot markers
- **Themes** - Built-in themes and customization options
- **Live code execution** - Driver configuration for SQL, shell, and custom languages
- **Animations** - Transitions, fragments, and incremental reveals
- **Code blocks** - Syntax highlighting, line highlighting, diffs
- **Mermaid diagrams** - Flowcharts, sequence diagrams, ER diagrams
- **AI images** - Gemini image generation from prompts
- **CLI commands** - All tap commands and their options
- **Best practices** - Presentation design tips

## Usage

Once installed, Claude Code automatically applies the skill when working on Tap presentations. You can ask Claude to:

- Create new presentations from scratch
- Add slides with specific layouts
- Set up live code execution for your database
- Configure themes and transitions
- Generate Mermaid diagrams
- Troubleshoot presentation issues

### Example Prompts

```
Create a presentation about our new API with live PostgreSQL demos
```

```
Add a two-column slide comparing React and Vue
```

```
Set up SQLite live code execution with an in-memory database
```

```
Convert these bullet points into incremental fragments
```

## Skill Source

The skill definitions are maintained in the Tap repository at [`skills/tap/`](https://github.com/tap-slides/tap/tree/main/skills/tap). You can review the rules or contribute improvements directly.

## Next Steps

- Read [Getting Started](/getting-started) if you're new to Tap
- Explore [Writing Slides](/guide/writing-slides) to learn the basics
- See [Live Code Execution](/guide/live-code-execution) for interactive demos
