---
title: Fences
---

# Fences keep their separators

```yaml
---
name: inside a backtick fence
---
```

---

~~~markdown
---
inside a tilde fence
~~~

---

````markdown
```yaml
---
```
````

---

- A list item with a fence

  ```yaml
  ---
  ```

---

<!-- skip: true -->

## A skipped slide

```sql {driver: sqlite, connection: demo} {2}
SELECT 1;
SELECT 2;
```
