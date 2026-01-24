# Code Blocks

Tap uses Shiki for syntax highlighting with support for hundreds of languages.

## Basic Syntax Highlighting

````markdown
```javascript
function greet(name) {
  return `Hello, ${name}!`;
}
```
````

### Common Languages

| Language | Identifier |
|----------|------------|
| JavaScript | `javascript`, `js` |
| TypeScript | `typescript`, `ts` |
| Python | `python`, `py` |
| Go | `go` |
| Rust | `rust` |
| SQL | `sql` |
| Bash/Shell | `bash`, `sh`, `shell` |
| HTML | `html` |
| CSS | `css` |
| JSON | `json` |
| YAML | `yaml` |

Use `text` or `plaintext` for no highlighting.

## Line Highlighting

Draw attention to specific lines:
````markdown
```javascript {2,4}
function calculate(a, b) {
  const sum = a + b;      // highlighted
  const diff = a - b;
  const product = a * b;  // highlighted
  return { sum, diff, product };
}
```
````

### Line Range Syntax

| Syntax | Meaning |
|--------|---------|
| `{5}` | Line 5 |
| `{1,3,5}` | Lines 1, 3, and 5 |
| `{1-3}` | Lines 1 through 3 |
| `{1-3,7,9-11}` | Combine ranges |

**Best Practice:** Highlight 1-3 lines at a time for clarity.

## Code Diffs

Show additions and removals:
````markdown
```diff
- const oldWay = require('module');
+ import newWay from 'module';

  function unchanged() {
-   return oldMethod();
+   return newMethod();
  }
```
````

Lines with `-` show as removals (red), `+` as additions (green).

## Multi-Step Code Reveals

Build code incrementally with pause:
````markdown
# Building a Function

```javascript
function processData(items) {
```

<!-- pause -->

```javascript
function processData(items) {
  const filtered = items.filter(x => x.active);
```

<!-- pause -->

```javascript
function processData(items) {
  const filtered = items.filter(x => x.active);
  const mapped = filtered.map(x => x.value);
  return mapped;
}
```
````

## Progressive Line Highlighting

Combine reveals with highlighting:
````markdown
```javascript {1-3}
async function fetchUser(id) {
  const response = await api.get(`/users/${id}`);
  return response.data;
}
```

<!-- pause -->

Now with error handling:

```javascript {4-8}
async function fetchUser(id) {
  try {
    const response = await api.get(`/users/${id}`);
    return response.data;
  } catch (error) {
    console.error('Failed:', error);
    throw error;
  }
}
```
````

## Font Size Configuration

Set in frontmatter:
```yaml
---
codeFontSize: 14px
---
```

| Size | Use Case |
|------|----------|
| `18px` | Large venue, few lines |
| `16px` | Default |
| `14px` | More code on screen |
| `12px` | Dense code |

**Tip:** Test at actual presentation distance.

## Quick Reference

| Feature | Syntax |
|---------|--------|
| Language | ` ```language ` |
| Single line | `{n}` |
| Multiple lines | `{n,m,o}` |
| Line range | `{n-m}` |
| Combined | `{n-m,o}` |
| Diff | ` ```diff ` |
| Font size | `codeFontSize` in frontmatter |
