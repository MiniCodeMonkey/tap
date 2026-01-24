# Live Code Execution

Execute code directly in slides with real-time results.

## Overview

Live code execution:
1. Executes code when the slide is displayed
2. Captures output (results, tables, text)
3. Renders output directly on the slide

**Important:** Only works with `tap dev`. Static builds show code but don't execute.

## Drivers

| Driver | Language | Use Case |
|--------|----------|----------|
| `sqlite` | SQL | SQLite database queries |
| `mysql` | SQL | MySQL/MariaDB queries |
| `postgres` | SQL | PostgreSQL queries |
| `shell` | Bash | Command-line scripts |
| Custom | Any | Python, Node.js, Ruby, etc. |

## Basic Syntax

Add `{driver: 'name'}` after the language:
````markdown
```sql {driver: sqlite, connection: demo}
SELECT * FROM users LIMIT 5;
```
````

## Database Configuration

### SQLite
```yaml
---
drivers:
  sqlite:
    connections:
      demo:
        path: ":memory:"  # or ./data/demo.db
---
```

### MySQL
```yaml
---
drivers:
  mysql:
    connections:
      default:
        host: localhost
        port: 3306
        database: myapp
        user: demo_user
        password: $MYSQL_PASSWORD
---
```

### PostgreSQL
```yaml
---
drivers:
  postgres:
    connections:
      analytics:
        host: localhost
        port: 5432
        database: analytics
        user: $PGUSER
        password: $PGPASSWORD
---
```

## Shell Driver

```yaml
---
drivers:
  shell:
    shell: /bin/bash
    cwd: ./demo-project
    timeout: 30
    env:
      NODE_ENV: development
---
```

Usage:
````markdown
```bash {driver: 'shell'}
ls -la | head -10
```
````

## Custom Drivers

Run any interpreter:
```yaml
---
drivers:
  python:
    command: python3
  node:
    command: node
  ruby:
    command: ruby
---
```

Usage:
````markdown
```python {driver: 'python'}
import json
data = {"name": "Tap", "version": "1.0"}
print(json.dumps(data, indent=2))
```
````

## Environment Variables

**Never hardcode passwords.** Use `$` prefix:
```yaml
---
drivers:
  postgres:
    connections:
      prod:
        host: localhost
        database: analytics
        user: $PGUSER
        password: $PGPASSWORD
---
```

Set before running:
```bash
export PGUSER=demo
export PGPASSWORD=secret123
tap dev slides.md
```

## Timeout Protection

Default: 10 seconds. Configure per driver:
```yaml
---
drivers:
  sqlite:
    timeout: 30
  shell:
    timeout: 60
---
```

Override per code block:
````markdown
```python {driver: 'python', timeout: '60'}
# Long-running computation
```
````

## Multiple Drivers

Use different drivers throughout:
```yaml
---
drivers:
  sqlite:
    connections:
      local:
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

````markdown
```sql {driver: sqlite, connection: local}
SELECT COUNT(*) FROM users;
```

---

```sql {driver: postgres, connection: analytics}
SELECT date, SUM(revenue) FROM sales GROUP BY date;
```

---

```bash {driver: 'shell'}
uname -a && df -h
```
````

## Best Practices

1. **Test queries** before presentation
2. **Use read-only credentials** for production databases
3. **Have a backup plan** if connectivity fails
4. **Keep queries fast**—audiences lose attention
5. **Use SQLite for portability**—no external server needed
