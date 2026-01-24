---
title: Tech Talk
---

# Tech Talk

A technical presentation template for conference talks and meetups.

## Overview

This example demonstrates how to structure a technical presentation with:

- Code blocks with syntax highlighting
- Live SQL demos with the SQLite driver
- Clean section breaks for flow
- Speaker notes for key talking points

## Features Used

- **Theme**: `paper` for clean, professional look
- **Layouts**: `title`, `section`, `code-focus`, `two-column`
- **Live Code**: SQLite driver for database demos
- **Fragments**: Incremental reveals for bullet points

## Source

```markdown
---
title: Building Scalable APIs
theme: paper
author: Your Name
date: 2026-01-24
aspectRatio: "16:9"
transition: fade
drivers:
  sqlite:
    connections:
      demo:
        path: ":memory:"
---

# Building Scalable APIs

Best practices for production-ready services

---

<!--
layout: section
-->

## The Problem

Why most APIs fail at scale

---

## Common Scaling Issues

<!-- pause -->

- **N+1 queries** — Database calls grow with data size

<!-- pause -->

- **Missing indexes** — Full table scans on every request

<!-- pause -->

- **No caching** — Recomputing the same results repeatedly

<!-- pause -->

- **Synchronous processing** — Blocking on slow operations

---

<!--
layout: two-column
-->

## Before & After

|||

### Before

```javascript
// N+1 query problem
for (const user of users) {
  const posts = await db.query(
    `SELECT * FROM posts
     WHERE user_id = ?`,
    [user.id]
  );
}
```

|||

### After

```javascript
// Single query with JOIN
const results = await db.query(`
  SELECT u.*, p.*
  FROM users u
  LEFT JOIN posts p
    ON u.id = p.user_id
`);
```

---

<!--
layout: section
-->

## Live Demo

Let's see the difference

---

## Create Sample Data

```sql {driver: sqlite, connection: demo}
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com'),
    ('Charlie', 'charlie@example.com');
```

---

## Query with Indexes

```sql {driver: sqlite, connection: demo}
CREATE INDEX idx_users_email ON users(email);

EXPLAIN QUERY PLAN
SELECT * FROM users WHERE email = 'alice@example.com';
```

---

<!--
layout: code-focus
-->

## Optimized Query

```sql {driver: sqlite, connection: demo}
SELECT
    name,
    email,
    (SELECT COUNT(*) FROM users) as total_users
FROM users
WHERE id = 1;
```

---

<!--
layout: quote
-->

> Premature optimization is the root of all evil, but ignoring obvious performance issues is just as bad.

Adapted from Donald Knuth

---

## Key Takeaways

<!-- pause -->

1. **Profile first** — Measure before optimizing

<!-- pause -->

2. **Index strategically** — Cover your common queries

<!-- pause -->

3. **Cache aggressively** — But invalidate correctly

<!-- pause -->

4. **Test at scale** — Production data sizes matter

---

<!--
layout: title
-->

# Questions?

@yourhandle | your@email.com
```

---

::: tip
This template works great for 20-45 minute conference talks. For shorter presentations, see the [Lightning Talk](/examples/lightning-talk) example.
:::
