---
title: SQL Live Demo
theme: noir
author: Database Expert
date: 2026-01-24
aspectRatio: "16:9"
transition: fade
codeTheme: github-light
fragments: true
drivers:
  sqlite:
    connections:
      demo:
        path: ":memory:"
---

# SQL Live Demo

Interactive database queries in your presentations

---

## SQLite in Memory

Tap includes built-in SQLite support for quick demos:

```sql {driver: sqlite, connection: demo}
SELECT sqlite_version() as version;
```

Perfect for teaching SQL or demonstrating queries.

---

<!--
layout: section
-->

## Creating Tables

Let's build a sample database

---

## Create the Users Table

```sql {driver: sqlite, connection: demo}
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

---

## Insert Sample Data

```sql {driver: sqlite, connection: demo}
INSERT INTO users (name, email) VALUES
    ('Alice Johnson', 'alice@example.com'),
    ('Bob Smith', 'bob@example.com'),
    ('Charlie Brown', 'charlie@example.com'),
    ('Diana Prince', 'diana@example.com'),
    ('Eve Wilson', 'eve@example.com');
```

---

## Query the Data

```sql {driver: sqlite, connection: demo}
SELECT id, name, email
FROM users
ORDER BY name;
```

---

<!--
layout: section
-->

## Advanced Queries

SQL features in action

---

## Filtering with WHERE

```sql {driver: sqlite, connection: demo}
SELECT name, email
FROM users
WHERE name LIKE '%o%'
ORDER BY name;
```

---

## Aggregation

```sql {driver: sqlite, connection: demo}
SELECT
    COUNT(*) as total_users,
    MIN(name) as first_name,
    MAX(name) as last_name
FROM users;
```

---

<!--
layout: two-column
-->

## Create Products Table

```sql {driver: sqlite, connection: demo}
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    price REAL NOT NULL,
    category TEXT
);
```

|||

### Table Schema

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Product name |
| price | REAL | Price in USD |
| category | TEXT | Category |

---

## Add Products

```sql {driver: sqlite, connection: demo}
INSERT INTO products (name, price, category) VALUES
    ('Laptop', 999.99, 'Electronics'),
    ('Headphones', 149.99, 'Electronics'),
    ('Coffee Mug', 12.99, 'Kitchen'),
    ('Notebook', 8.99, 'Office'),
    ('Desk Lamp', 45.99, 'Office'),
    ('Wireless Mouse', 29.99, 'Electronics');
```

---

## Products by Category

```sql {driver: sqlite, connection: demo}
SELECT
    category,
    COUNT(*) as item_count,
    ROUND(AVG(price), 2) as avg_price,
    ROUND(SUM(price), 2) as total_value
FROM products
GROUP BY category
ORDER BY total_value DESC;
```

---

<!--
layout: code-focus
-->

## Complex Queries

```sql {driver: sqlite, connection: demo}
-- Find expensive items above average price
WITH avg_price AS (
    SELECT AVG(price) as avg FROM products
)
SELECT
    name,
    printf("$%.2f", price) as price,
    category
FROM products, avg_price
WHERE price > avg_price.avg
ORDER BY price DESC;
```

---

## Creating Orders

```sql {driver: sqlite, connection: demo}
CREATE TABLE orders (
    id INTEGER PRIMARY KEY,
    user_id INTEGER,
    product_id INTEGER,
    quantity INTEGER DEFAULT 1,
    order_date TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);
```

---

## Sample Orders

```sql {driver: sqlite, connection: demo}
INSERT INTO orders (user_id, product_id, quantity) VALUES
    (1, 1, 1),  -- Alice bought Laptop
    (1, 2, 2),  -- Alice bought 2 Headphones
    (2, 3, 3),  -- Bob bought 3 Coffee Mugs
    (3, 4, 5),  -- Charlie bought 5 Notebooks
    (4, 5, 1),  -- Diana bought Desk Lamp
    (5, 6, 2);  -- Eve bought 2 Wireless Mice
```

---

<!--
layout: code-focus
-->

## Joining Tables

```sql {driver: sqlite, connection: demo}
SELECT
    u.name as customer,
    p.name as product,
    o.quantity,
    printf("$%.2f", p.price * o.quantity) as total
FROM orders o
JOIN users u ON o.user_id = u.id
JOIN products p ON o.product_id = p.id
ORDER BY total DESC;
```

---

## Customer Spending

```sql {driver: sqlite, connection: demo}
SELECT
    u.name as customer,
    COUNT(o.id) as orders,
    printf("$%.2f", SUM(p.price * o.quantity)) as total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
LEFT JOIN products p ON o.product_id = p.id
GROUP BY u.id
ORDER BY SUM(p.price * o.quantity) DESC;
```

---

<!--
layout: big-stat
-->

## 100%

of queries execute live in the browser

---

## Why Live SQL?

<!-- pause -->

- **Engage your audience** with real-time results

<!-- pause -->

- **Demonstrate concepts** step by step

<!-- pause -->

- **Handle questions** with ad-hoc queries

<!-- pause -->

- **No pre-recorded screenshots** that get outdated

---

<!--
layout: title
-->

# SQL Made Interactive

Transform your database presentations
