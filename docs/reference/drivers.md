---
title: Drivers
---

# Drivers

Complete reference for all code execution drivers in Tap. Drivers enable live code execution during presentations using `tap dev`.

::: warning Development Mode Only
Live code execution only works in development mode (`tap dev`). Static builds show code blocks but don't execute them.
:::

## Overview

Drivers are configured in the presentation frontmatter under the `drivers` key. Database drivers use named connections, while shell and custom drivers use direct configuration.

```yaml
---
title: My Presentation
drivers:
  sqlite:
    connections:
      demo:
        path: ":memory:"
  mysql:
    connections:
      default:
        host: localhost
        database: myapp
        user: demo_user
  postgres:
    connections:
      analytics:
        host: localhost
        database: analytics
        user: readonly
  shell:
    timeout: 30
---
```

## SQLite Driver

The SQLite driver executes SQL queries against a SQLite database file or in-memory database.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `path` | `string` | `:memory:` | Path to SQLite database file, or `:memory:` for in-memory |

### Frontmatter Configuration

SQLite uses named connections under the `connections` key:

```yaml
---
drivers:
  sqlite:
    connections:
      demo:
        path: ":memory:"
      production:
        path: "./data/demo.db"
---
```

### Usage

Reference the connection name in your code block:

````markdown
```sql {driver: sqlite, connection: demo}
SELECT * FROM users WHERE active = 1 LIMIT 10;
```
````

### In-Memory Database

Use `:memory:` as the path for a fresh in-memory database. This is useful for demos where you create tables and data on the fly:

````markdown
```sql {driver: sqlite, connection: demo}
CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL);
INSERT INTO products VALUES (1, 'Widget', 9.99), (2, 'Gadget', 19.99);
SELECT * FROM products;
```
````

::: tip Best for Portability
SQLite requires no external database server, making it ideal for portable presentations that work on any machine.
:::

---

## MySQL Driver

The MySQL driver executes SQL queries against a MySQL or MariaDB database server.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | `string` | `localhost` | Database server hostname |
| `port` | `number` | `3306` | Database server port |
| `database` | `string` | *required* | Database name |
| `user` | `string` | *required* | Username for authentication |
| `password` | `string` | `""` | Password for authentication |

### Frontmatter Configuration

MySQL uses named connections under the `connections` key:

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

### Usage

Reference the connection name in your code block:

````markdown
```sql {driver: mysql, connection: default}
SELECT
  department,
  COUNT(*) as employee_count,
  AVG(salary) as avg_salary
FROM employees
GROUP BY department
ORDER BY avg_salary DESC;
```
````

### Environment Variables

Use environment variables for credentials to avoid hardcoding sensitive data:

```yaml
---
drivers:
  mysql:
    connections:
      production:
        host: $MYSQL_HOST
        database: $MYSQL_DATABASE
        user: $MYSQL_USER
        password: $MYSQL_PASSWORD
---
```

Set the variables before running:

```bash
export MYSQL_HOST=db.example.com
export MYSQL_DATABASE=production
export MYSQL_USER=readonly
export MYSQL_PASSWORD=secret123
tap dev slides.md
```

---

## PostgreSQL Driver

The PostgreSQL driver executes SQL queries against a PostgreSQL database server.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | `string` | `localhost` | Database server hostname |
| `port` | `number` | `5432` | Database server port |
| `database` | `string` | *required* | Database name |
| `user` | `string` | *required* | Username for authentication |
| `password` | `string` | `""` | Password for authentication |

### Frontmatter Configuration

PostgreSQL uses named connections under the `connections` key:

```yaml
---
drivers:
  postgres:
    connections:
      analytics:
        host: localhost
        port: 5432
        database: analytics
        user: demo_user
        password: $PGPASSWORD
---
```

### Usage

Reference the connection name in your code block:

````markdown
```sql {driver: postgres, connection: analytics}
SELECT
  date_trunc('month', created_at) as month,
  SUM(amount) as total_revenue
FROM orders
WHERE created_at >= NOW() - INTERVAL '1 year'
GROUP BY month
ORDER BY month;
```
````

### Standard PostgreSQL Environment Variables

PostgreSQL driver also respects standard PostgreSQL environment variables:

| Environment Variable | Maps To |
|---------------------|---------|
| `PGHOST` | `host` |
| `PGPORT` | `port` |
| `PGDATABASE` | `database` |
| `PGUSER` | `user` |
| `PGPASSWORD` | `password` |

If you have these set, you can use a minimal configuration:

```yaml
---
drivers:
  postgres:
    connections:
      default: {}  # Uses PGHOST, PGDATABASE, etc.
---
```

---

## Shell Driver

The Shell driver executes shell commands and scripts, displaying the output on your slides.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `shell` | `string` | System default | Shell interpreter path (e.g., `/bin/bash`, `/bin/zsh`) |
| `cwd` | `string` | Presentation directory | Working directory for commands |
| `timeout` | `number` | `10` | Command timeout in seconds |
| `env` | `object` | `{}` | Additional environment variables |

### Frontmatter Configuration

```yaml
---
drivers:
  shell:
    shell: /bin/bash
    cwd: ./demo-project
    timeout: 30
    env:
      NODE_ENV: development
      DEBUG: "true"
---
```

### Usage

````markdown
```bash {driver: 'shell'}
ls -la | head -10
```
````

### Running Scripts

Execute scripts in your project:

````markdown
```bash {driver: 'shell'}
./scripts/generate-report.sh
```
````

### Running Other Languages

Use the shell driver to run any language interpreter:

````markdown
```python {driver: 'shell'}
python3 << 'EOF'
import json
data = {"name": "Tap", "version": "1.0"}
print(json.dumps(data, indent=2))
EOF
```
````

````markdown
```javascript {driver: 'shell'}
node -e "console.log(JSON.stringify({timestamp: Date.now()}, null, 2))"
```
````

### Working Directory

The `cwd` option is useful when your commands need to run from a specific directory:

```yaml
---
drivers:
  shell:
    cwd: ./my-app
---
```

Now commands run relative to `./my-app`:

````markdown
```bash {driver: 'shell'}
npm test          # Runs in ./my-app
cat package.json  # Reads ./my-app/package.json
```
````

### Custom Environment

Pass environment variables to your commands:

```yaml
---
drivers:
  shell:
    env:
      API_URL: https://api.example.com
      LOG_LEVEL: debug
---
```

---

## Common Configuration

### Timeout Protection

All drivers support the `timeout` option to prevent runaway queries or infinite loops:

```yaml
---
drivers:
  sqlite:
    timeout: 30  # 30 seconds
  shell:
    timeout: 60  # 60 seconds for longer operations
---
```

When a timeout is reached:
1. The execution is cancelled
2. A timeout error is displayed on the slide
3. The presentation continues normally

### Environment Variable Substitution

All driver configuration values support environment variable substitution using `$` prefix:

```yaml
---
drivers:
  postgres:
    host: $DB_HOST
    database: $DB_NAME
    user: $DB_USER
    password: $DB_PASSWORD
---
```

This keeps sensitive credentials out of your presentation files.

---

## Quick Reference

### Driver Types

| Driver | Language | Typical Use |
|--------|----------|-------------|
| `sqlite` | SQL | Local/portable demos, in-memory databases |
| `mysql` | SQL | MySQL/MariaDB database queries |
| `postgres` | SQL | PostgreSQL database queries |
| `shell` | Bash/Shell | Command-line demos, running scripts |

### Configuration Summary

| Driver | Required Options | Key Optional Options |
|--------|------------------|---------------------|
| `sqlite` | `connections.<name>.path` | - |
| `mysql` | `connections.<name>.database`, `user` | `host`, `port`, `password` |
| `postgres` | `connections.<name>.database`, `user` | `host`, `port`, `password` |
| `shell` | None | `shell`, `cwd`, `env`, `timeout` |
| Custom | `command` | `args`, `timeout` |

### Code Block Syntax

For database drivers, specify both driver and connection:

````markdown
```sql {driver: sqlite, connection: demo}
SELECT * FROM users;
```
````

For shell and custom drivers:

````markdown
```bash {driver: shell}
ls -la
```

```python {driver: python}
print("Hello, World!")
```
````

---

## Custom Drivers

Custom drivers allow you to execute code using any command-line tool or interpreter. This enables support for any language or tool not covered by the built-in drivers.

### How It Works

Custom drivers pass code to a command via stdin. The command's stdout becomes the result displayed on your slide. This simple model supports virtually any language or tool.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `command` | `string` | *required* | The executable to run (e.g., `python`, `node`, `ruby`) |
| `args` | `array` | `[]` | Arguments passed to the command before code is sent via stdin |
| `timeout` | `number` | `30` | Execution timeout in seconds |

### Frontmatter Configuration

Define custom drivers in your frontmatter using any name (except reserved names: `shell`, `sqlite`, `mysql`, `postgres`):

```yaml
---
title: Multi-Language Demo
drivers:
  python:
    command: python3
  node:
    command: node
  ruby:
    command: ruby
    timeout: 15
---
```

### Usage Examples

#### Python

````markdown
```python {driver: 'python'}
import json
data = {"name": "Tap", "version": "1.0"}
print(json.dumps(data, indent=2))
```
````

#### Node.js

````markdown
```javascript {driver: 'node'}
const os = require('os');
console.log(`Platform: ${os.platform()}`);
console.log(`Architecture: ${os.arch()}`);
console.log(`CPUs: ${os.cpus().length}`);
```
````

#### Ruby

````markdown
```ruby {driver: 'ruby'}
require 'json'
data = { name: 'Tap', languages: ['Go', 'JavaScript'] }
puts JSON.pretty_generate(data)
```
````

### Advanced: Custom Arguments

Use `args` to pass flags to the interpreter:

```yaml
---
drivers:
  python:
    command: python3
    args: ["-u"]  # Unbuffered output
  node:
    command: node
    args: ["--experimental-modules"]
---
```

### Code Block Timeout Override

Override the driver's default timeout for a specific code block:

````markdown
```python {driver: 'python', timeout: '60'}
# Long-running computation...
import time
time.sleep(5)
print("Done!")
```
````

::: warning Availability
Custom driver commands must be installed and available in the system PATH where `tap dev` is running.
:::

---

## Next Steps

- Learn the basics of live code execution in the [Live Code Execution Guide](/guide/live-code-execution)
- Configure presentation settings in [Frontmatter Options](/reference/frontmatter-options)
- See code display options in [Code Blocks](/guide/code-blocks)
