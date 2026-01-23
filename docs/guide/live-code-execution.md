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

::: warning Development Mode Only
Live code execution only works when using `tap dev`. Static builds created with `tap build` will show the code blocks but won't execute them. This is by design for security and portability.
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

## Connection Configuration

For database drivers, you configure connections in the frontmatter. This keeps credentials and connection details at the top of your presentation file.

### SQLite

SQLite is the simplest—just specify the database file:

```yaml
---
title: Database Demo
drivers:
  sqlite:
    database: ./data/demo.db
---
```

If no database is specified, Tap uses an in-memory SQLite database.

### MySQL

```yaml
---
title: MySQL Demo
drivers:
  mysql:
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
    host: localhost
    port: 5432
    database: analytics
    user: demo_user
    sslmode: prefer
---
```

## Environment Variables for Credentials

**Never hardcode passwords in your presentation files.** Use environment variables for sensitive credentials:

```yaml
---
drivers:
  postgres:
    host: localhost
    database: analytics
    user: $PGUSER
    password: $PGPASSWORD
---
```

Values starting with `$` are replaced with the corresponding environment variable. Set them before running Tap:

```bash
export PGUSER=demo
export PGPASSWORD=secret123
tap dev slides.md
```

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
    database: ./demo.db
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
    database: ./app.db
  postgres:
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

```sql {driver: 'sqlite'}
SELECT COUNT(*) FROM users;
```

---

# Analytics

```sql {driver: 'postgres'}
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
| `$ENV_VAR` in config | Use environment variable |
| `timeout: N` | Set timeout in seconds |

## Best Practices

1. **Test your queries** before the presentation—run through all slides once
2. **Use read-only credentials** when connecting to production databases
3. **Have a backup plan** if network/database connectivity fails
4. **Keep queries fast**—audiences lose attention during long-running operations
5. **Use SQLite for portability**—it requires no external database server

## Next Steps

- Learn about all driver options in the [Drivers Reference](/reference/drivers)
- See how to style code output with [Code Blocks](/guide/code-blocks)
- Configure presentation-wide settings in [Frontmatter Options](/reference/frontmatter-options)
