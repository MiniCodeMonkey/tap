---
title: Debugging Production at 3am
theme: terminal
author: On-Call Engineer
date: 2026-01-25
aspectRatio: "16:9"
transition: fade
drivers:
  sqlite:
    connections:
      incident:
        database: ":memory:"
---

<!-- layout: title -->

# Debugging Production at 3am

What the pager doesn't tell you

---

<!-- layout: section -->

# The Page

---

## What We Knew

- Error rate spiked at 02:47
- No deploy in the last 6 hours
- One region only

<!-- pause -->

Nothing in the runbook covered this.

---

<!--
layout: code-focus
-->

```sql {driver: sqlite, connection: incident} {2-3}
SELECT region, count(*) AS errors
FROM request_log
WHERE status >= 500 AND ts > datetime('now', '-1 hour')
GROUP BY region
ORDER BY errors DESC;
```

---

<!--
layout: two-column
-->

## Root Cause

### What we suspected

- Bad deploy
- Database failover
- Certificate expiry

::right

### What it actually was

- A cron job's retry loop
- Hammering a single connection pool
- For six weeks, unnoticed

---

<!--
layout: big-stat
-->

# 11 min

::caption

From page to root cause, once we had the right query

---

<!-- layout: section -->

# What Changed

---

## Three Fixes

- Connection pool alerts on saturation, not just errors
- The cron job now backs off instead of retrying immediately
- The runbook has this incident in it now

---

<!--
layout: quote
-->

> The system was never down. It was just very, very busy.

::attribution

Postmortem, paraphrased
