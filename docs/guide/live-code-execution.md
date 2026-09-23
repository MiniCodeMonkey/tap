---
title: Live Code Execution
---

# Live Code Execution

One of Tap's most powerful features is the ability to execute code directly in your slides. Instead of showing static code snippets, you can run queries, scripts, and commands and display the results live during your presentation.

## What is Live Code Execution?

Live code execution transforms your slides from static content into interactive demonstrations. When you mark a code block for execution, Tap will:

1. Execute the code when the slide is displayed
2. Capture the output (results, tables, or text)
3. Render the output directly on the slide

This is perfect for:
- Database demos with real query results
- CLI tool demonstrations
- Data analysis presentations
- Teaching programming concepts
- System administration tutorials

::: warning Not in Static Builds
Live code execution works when a server is running the deck, with `tap dev` or `tap present`. Static builds created with `tap build` (or `tap export`) will show the code blocks but won't execute them. This is by design for security and portability.
:::

## The Driver Concept

Tap uses **drivers** to execute code. A driver is a connector that knows how to run a specific type of code and format the results. When you want code to execute, you specify which driver should handle it.

Built-in drivers include:

| Driver | Language | Use Case |
|--------|----------|----------|
| `sqlite` | SQL | SQLite database queries |
| `mysql` | SQL | MySQL/MariaDB database queries |
| `postgres` | SQL | PostgreSQL database queries |
| `shell` | Bash/Shell | Command-line scripts |

## Basic Syntax

To make a code block executable, add the `driver` annotation after the language identifier:

````markdown
```sql {driver: 'sqlite'}
SELECT * FROM users LIMIT 5;
```
````

The code block will execute using the specified driver and display results below the code.

### Example: SQLite Query

````markdown
```sql {driver: 'sqlite'}
SELECT
  name,
  department,
  salary
FROM employees
WHERE department = 'Engineering'
ORDER BY salary DESC
LIMIT 10;
```
````

Results are displayed as a formatted table directly on the slide.

### Example: Shell Command

````markdown
```bash {driver: 'shell'}
ls -la | head -10
```
````

Shell output is rendered with syntax highlighting.

## Declare the drivers a deck uses

Every driver a live code block uses must be a key under `drivers:` in the frontmatter. A driver with no settings is declared as `{}`:

```yaml
---
title: My Talk
drivers:
  shell: {}
  sqlite:
    connections:
      demo:
        path: ":memory:"
---
```

A block whose driver is not declared never runs. The block shows what to add, and `tap dev` prints the same message with the file and line:

```
warning: talk.md:24: This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.
```

## Approving a deck

A deck with live code runs nothing until you approve it. The first time `tap dev` or `tap present` opens it in a terminal, tap asks before the TUI starts:

```
This deck can run code on this computer:
  /Users/me/talks/talk.md

  python     1 block on slide 7, runs: python3 -c
  shell      2 blocks on slides 3, 5

A yes is remembered for this deck, so every future run skips this question; undo it with tap approval revoke /Users/me/talks/talk.md.

Allow this deck to run code? Type s to show the code. [y/N/s]
```

- `s` prints every block, then asks again. Return means no.
- A yes is saved in `~/.config/tap/settings.yaml` with the deck's path and its drivers. Editing the code never asks again.
- A no saves nothing. The deck still previews and presents, its Run buttons show "Not approved", and tap asks again next time.
- A new driver in the frontmatter asks again, and names only the new driver. A moved deck asks again, because approvals are keyed by path, not by the deck's content. This cuts both ways: replace the file at an approved path with a different deck, and if that deck's drivers are already covered by the approval, tap runs its code without asking again. Only overwrite an approved path with a deck you trust.
- `tap new` approves the deck it creates.
- Without a terminal, or with `--headless`, tap never asks. An unapproved deck's live code stays off. `--allow-code` turns it on for that run and saves nothing.
- `tap approval list` shows the approved decks, and `tap approval revoke <deck>` removes one.

## What tap runs

A Run button sends only the slide number and the block number, such as `{"slide": 4, "block": 1}`. tap runs the code the deck file holds at that position, with that block's driver and connection. `/api/execute` refuses a request that carries code (400), an unknown slide or block (404), a block whose driver the deck does not declare (422), and a driver this run has not approved (403). `tap build` and `tap export` never run code.

## Connection Configuration

For database drivers, you configure connections in the frontmatter. This keeps credentials and connection details at the top of your presentation file.

### SQLite

SQLite is the simplest: just specify the database file with `path`.

```yaml
---
title: Database Demo
drivers:
  sqlite:
    connections:
      demo:
        path: ./data/demo.db
---
```

If no path is specified, Tap uses an in-memory SQLite database. `database` is an older spelling of `path`, kept working for decks that already use it; `path` wins when both are set. New decks should use `path`.

### MySQL

```yaml
---
title: MySQL Demo
drivers:
  mysql:
    connections:
      demo:
        host: localhost
        port: 3306
        database: myapp
        user: demo_user
---
```

### PostgreSQL

```yaml
---
title: PostgreSQL Demo
drivers:
  postgres:
    connections:
      demo:
        host: localhost
        port: 5432
        database: analytics
        user: demo_user
---
```

### Environment variables

String values in `drivers:` settings can read the environment with `${NAME}`:

```yaml
drivers:
  postgres:
    connections:
      demo:
        host: ${PGHOST}
        user: ${PGUSER}
        password: ${PGPASSWORD}
```

- tap expands `${NAME}` when a block runs, not when it loads the deck, so the value never reaches the slide page or a `tap build` folder.
- A `.env` file next to the deck is read too.
- A variable that is not set makes the block fail with a message that names it. It never becomes an empty string.
- `$${` writes a literal `${`. Any other `$` stays as it is, so `$PGPASSWORD` without braces is not expanded.
- Only driver settings expand. Other frontmatter keys, such as `title`, stay as written.

::: tip Credential Management
For team presentations, consider using a `.env` file (excluded from version control) or your organization's secrets management solution.
:::

## Timeout Protection

To prevent runaway queries or infinite loops from freezing your presentation, Tap enforces execution timeouts:

- Default timeout: **10 seconds**
- Configurable per driver in frontmatter

```yaml
---
drivers:
  sqlite:
    connections:
      demo:
        path: ./demo.db
    timeout: 30  # seconds
---
```

If a query exceeds the timeout, Tap will:
1. Cancel the execution
2. Display a timeout error on the slide
3. Allow you to continue with the presentation

## Error Handling

When code execution fails, Tap displays the error message directly on the slide instead of crashing. This allows you to:

- Debug issues during development
- Gracefully handle unexpected errors during live presentations
- Show intentional errors for teaching purposes

Error output is styled distinctly (typically red text) so it's clear something went wrong.

### Example Error Display

If you run an invalid query:

````markdown
```sql {driver: 'sqlite'}
SELECT * FROM nonexistent_table;
```
````

The slide will show:
```
Error: no such table: nonexistent_table
```

## Shell Driver Options

The shell driver has additional configuration options:

```yaml
---
drivers:
  shell:
    shell: /bin/bash        # Shell to use (default: system default)
    cwd: ./scripts          # Working directory
    timeout: 30             # Timeout in seconds
    env:                    # Additional environment variables
      NODE_ENV: development
---
```

### Running Different Interpreters

You can run any interpreter using the shell driver:

````markdown
```python {driver: 'shell'}
python3 -c "
import json
data = {'name': 'Tap', 'version': '1.0'}
print(json.dumps(data, indent=2))
"
```
````

## Multiple Drivers in One Presentation

You can configure multiple drivers and use different ones throughout your presentation:

```yaml
---
title: Full Stack Demo
drivers:
  sqlite:
    connections:
      demo:
        path: ./app.db
  postgres:
    connections:
      analytics:
        host: localhost
        database: analytics
  shell:
    cwd: ./demo
---
```

Then use the appropriate driver for each code block:

````markdown
---

# Local Database

```sql {driver: 'sqlite', connection: 'demo'}
SELECT COUNT(*) FROM users;
```

---

# Analytics

```sql {driver: 'postgres', connection: 'analytics'}
SELECT date, SUM(revenue) FROM sales GROUP BY date;
```

---

# System Info

```bash {driver: 'shell'}
uname -a && df -h
```
````

## Quick Reference

| Syntax | Description |
|--------|-------------|
| `{driver: 'sqlite'}` | Execute with SQLite driver |
| `{driver: 'mysql'}` | Execute with MySQL driver |
| `{driver: 'postgres'}` | Execute with PostgreSQL driver |
| `{driver: 'shell'}` | Execute with shell driver |
| `${ENV_VAR}` in a driver setting | Use environment variable |
| `timeout: N` | Set timeout in seconds |

## Best Practices

1. **Test your queries** before the presentation: run through all slides once
2. **Use read-only credentials** when connecting to production databases
3. **Have a backup plan** if network/database connectivity fails
4. **Keep queries fast.** Audiences lose attention during long-running operations
5. **Use SQLite for portability.** It requires no external database server

## Next Steps

- Learn about all driver options in the [Drivers Reference](/reference/drivers)
- See how to style code output with [Code Blocks](/guide/code-blocks)
- Configure presentation-wide settings in [Frontmatter Options](/reference/frontmatter-options)
