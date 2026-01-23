---
title: Drivers
---

# Drivers

Complete reference for all code execution drivers in Tap. Drivers enable live code execution during presentations using `tap dev`.

::: warning Development Mode Only
Live code execution only works in development mode (`tap dev`). Static builds show code blocks but don't execute them.
:::

## Overview

Drivers are configured in the presentation frontmatter under the `drivers` key. Each driver type has its own configuration options.

```yaml
---
title: My Presentation
drivers:
  sqlite:
    database: ./demo.db
  mysql:
    host: localhost
    database: myapp
  postgres:
    host: localhost
    database: analytics
  shell:
    cwd: ./scripts
---
```

## SQLite Driver

The SQLite driver executes SQL queries against a SQLite database file or in-memory database.

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `database` | `string` | `:memory:` | Path to SQLite database file, or `:memory:` for in-memory |
| `timeout` | `number` | `10` | Query timeout in seconds |
| `readonly` | `boolean` | `false` | Open database in read-only mode |

### Frontmatter Configuration

```yaml
---
drivers:
  sqlite:
    database: ./data/demo.db
    timeout: 30
    readonly: true
---
```

### Usage

````markdown
```sql {driver: 'sqlite'}
SELECT * FROM users WHERE active = 1 LIMIT 10;
```
````

### In-Memory Database

When no database is specified (or set to `:memory:`), Tap creates a fresh in-memory SQLite database. This is useful for demos where you create tables and data on the fly:

````markdown
```sql {driver: 'sqlite'}
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
| `timeout` | `number` | `10` | Query timeout in seconds |
| `charset` | `string` | `utf8mb4` | Character set for connection |
| `ssl` | `boolean` | `false` | Enable SSL/TLS connection |

### Frontmatter Configuration

```yaml
---
drivers:
  mysql:
    host: localhost
    port: 3306
    database: myapp
    user: demo_user
    password: $MYSQL_PASSWORD
    timeout: 15
    charset: utf8mb4
---
```

### Usage

````markdown
```sql {driver: 'mysql'}
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
| `timeout` | `number` | `10` | Query timeout in seconds |
| `sslmode` | `string` | `prefer` | SSL mode: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |
| `schema` | `string` | `public` | Default schema to use |

### Frontmatter Configuration

```yaml
---
drivers:
  postgres:
    host: localhost
    port: 5432
    database: analytics
    user: demo_user
    password: $PGPASSWORD
    sslmode: require
    schema: public
    timeout: 20
---
```

### Usage

````markdown
```sql {driver: 'postgres'}
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
| `PGSSLMODE` | `sslmode` |

If you have these set, you can use a minimal configuration:

```yaml
---
drivers:
  postgres: {}  # Uses PGHOST, PGDATABASE, etc.
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
| `sqlite` | None | `database`, `timeout`, `readonly` |
| `mysql` | `database`, `user` | `host`, `port`, `password`, `ssl` |
| `postgres` | `database`, `user` | `host`, `port`, `password`, `sslmode`, `schema` |
| `shell` | None | `shell`, `cwd`, `env`, `timeout` |

### Code Block Syntax

````markdown
```language {driver: 'driver_name'}
code here
```
````

---

## Custom Drivers

::: tip Coming Soon
Support for custom drivers is planned for a future release. Custom drivers will allow you to:
- Integrate with additional databases (MongoDB, Redis, etc.)
- Connect to REST APIs and display formatted results
- Run language-specific REPLs (Python, Ruby, Node.js)
- Create specialized output formatters

Watch the [GitHub repository](https://github.com/tap-slides/tap) for updates.
:::

---

## Next Steps

- Learn the basics of live code execution in the [Live Code Execution Guide](/guide/live-code-execution)
- Configure presentation settings in [Frontmatter Options](/reference/frontmatter-options)
- See code display options in [Code Blocks](/guide/code-blocks)
