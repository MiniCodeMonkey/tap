---
title: Custom Driver
drivers:
  sqlite: {}
  fortune:
    command: /bin/cat
---

# One

---

# Query

```sql {driver: sqlite}
SELECT 1 AS one;
```

---

# Fortune

```text {driver: fortune}
hello from cat
```

---

# Shell

```bash {driver: shell}
echo four
```
