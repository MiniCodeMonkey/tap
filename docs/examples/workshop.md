---
title: Workshop
---

# Workshop

An interactive workshop template with hands-on exercises.

## Overview

This example demonstrates how to structure an interactive workshop with:

- Learning objectives upfront
- Step-by-step instructions
- Code examples to follow along
- Exercise checkpoints

## Features Used

- **Theme**: `paper` for readability during exercises
- **Layouts**: `title`, `two-column`, `code-focus`, `section`
- **Live Code**: SQLite driver for hands-on database exercises
- **Fragments**: Step-by-step reveals for instructions

## Source

````markdown
---
title: SQL Fundamentals Workshop
theme: paper
author: Workshop Instructor
date: 2026-01-24
aspectRatio: "16:9"
transition: fade
fragments: true
drivers:
  sqlite:
    connections:
      workshop:
        path: ":memory:"
---

# SQL Fundamentals

A hands-on introduction to database queries

---

## Learning Objectives

By the end of this workshop, you will be able to:

<!-- pause -->

- Create tables and define schemas

<!-- pause -->

- Insert, update, and delete data

<!-- pause -->

- Write SELECT queries with filters

<!-- pause -->

- Join tables and aggregate results

---

<!--
layout: section
-->

## Part 1

Creating Your First Table

---

## Step 1: Create a Table

Let's create a simple `books` table:

```sql {driver: sqlite, connection: workshop}
CREATE TABLE books (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    year INTEGER,
    pages INTEGER
);
```

---

## Step 2: Insert Data

Add some books to our table:

```sql {driver: sqlite, connection: workshop}
INSERT INTO books (title, author, year, pages) VALUES
    ('The Pragmatic Programmer', 'David Thomas', 1999, 352),
    ('Clean Code', 'Robert C. Martin', 2008, 464),
    ('Design Patterns', 'Gang of Four', 1994, 416),
    ('Refactoring', 'Martin Fowler', 2018, 448);
```

---

## Step 3: Query the Data

Retrieve all books:

```sql {driver: sqlite, connection: workshop}
SELECT * FROM books;
```

---

<!--
layout: two-column
-->

## Exercise 1

|||

### Your Task

Write a query to find:
- Books with more than 400 pages
- Sort by year (newest first)

<!-- pause -->

### Hints

- Use `WHERE` for filtering
- Use `ORDER BY` for sorting
- Use `DESC` for descending order

|||

### Solution

```sql {driver: sqlite, connection: workshop}
SELECT title, author, year, pages
FROM books
WHERE pages > 400
ORDER BY year DESC;
```

---

<!--
layout: section
-->

## Part 2

Filtering and Aggregation

---

## WHERE Clause

Filter rows based on conditions:

```sql {driver: sqlite, connection: workshop}
SELECT title, author
FROM books
WHERE year >= 2000;
```

---

## Aggregate Functions

Count, sum, and average:

```sql {driver: sqlite, connection: workshop}
SELECT
    COUNT(*) as total_books,
    AVG(pages) as avg_pages,
    MIN(year) as oldest,
    MAX(year) as newest
FROM books;
```

---

<!--
layout: code-focus
-->

## GROUP BY

Aggregate by category:

```sql {driver: sqlite, connection: workshop}
SELECT
    author,
    COUNT(*) as book_count,
    SUM(pages) as total_pages
FROM books
GROUP BY author;
```

---

<!--
layout: two-column
-->

## Exercise 2

|||

### Your Task

Find the average page count for books published:
- Before 2000
- After 2000

Compare the results.

|||

### Solution

```sql {driver: sqlite, connection: workshop}
SELECT
    CASE
        WHEN year < 2000 THEN 'Pre-2000'
        ELSE 'Post-2000'
    END as era,
    ROUND(AVG(pages), 0) as avg_pages
FROM books
GROUP BY era;
```

---

<!--
layout: section
-->

## Part 3

Joining Tables

---

## Create a Second Table

Add a `reviews` table:

```sql {driver: sqlite, connection: workshop}
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY,
    book_id INTEGER,
    rating INTEGER,
    reviewer TEXT,
    FOREIGN KEY (book_id) REFERENCES books(id)
);

INSERT INTO reviews (book_id, rating, reviewer) VALUES
    (1, 5, 'Alice'), (1, 4, 'Bob'),
    (2, 5, 'Charlie'), (2, 5, 'Diana'),
    (3, 4, 'Eve'), (4, 5, 'Frank');
```

---

## JOIN Tables

Combine data from both tables:

```sql {driver: sqlite, connection: workshop}
SELECT
    b.title,
    r.rating,
    r.reviewer
FROM books b
JOIN reviews r ON b.id = r.book_id
ORDER BY b.title;
```

---

## Aggregate Joined Data

Average rating per book:

```sql {driver: sqlite, connection: workshop}
SELECT
    b.title,
    ROUND(AVG(r.rating), 1) as avg_rating,
    COUNT(r.id) as review_count
FROM books b
LEFT JOIN reviews r ON b.id = r.book_id
GROUP BY b.id
ORDER BY avg_rating DESC;
```

---

## Workshop Complete!

### Key Concepts Covered

- `CREATE TABLE` — Define schema
- `INSERT` — Add data
- `SELECT` — Query data
- `WHERE` — Filter results
- `GROUP BY` — Aggregate data
- `JOIN` — Combine tables

---

<!--
layout: title
-->

# Practice Makes Perfect

Try these queries on your own data!
````

---

::: tip
Workshops benefit from frequent pauses. Use the `<!-- pause -->` directive to create natural stopping points where participants can catch up.
:::
