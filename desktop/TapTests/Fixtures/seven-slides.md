---
title: Seven Slides
drivers:
  sqlite: {}
---

<!-- layout: title -->

# Debugging Production at 3am

What the pager doesn't tell you

---

<!-- layout: sectoin -->

# The Page

---

## What We Knew

- Error rate spiked at 02:47
- No deploy in the last 6 hours

<!-- pause -->

Nothing in the runbook covered this.

<!--
notes:
Let the room guess the cause first.
-->

---

<!-- layout: code-focus -->

```sql {driver: sqlite}
SELECT region, count(*) AS errors FROM request_log GROUP BY region;
```

---

# Root Cause

---

# Eleven Minutes

---

# Tail the Logs
