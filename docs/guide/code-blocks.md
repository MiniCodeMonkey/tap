---
title: Code Blocks
---

# Code Blocks

Tap uses [Shiki](https://shiki.style/) for syntax highlighting, giving you beautiful, accurate code presentation with support for hundreds of languages.

## Basic Syntax Highlighting

Wrap your code in fenced code blocks and specify the language:

````markdown
```javascript
function greet(name) {
  return `Hello, ${name}!`;
}
```
````

Shiki provides editor-quality highlighting that accurately tokenizes your code, ensuring keywords, strings, comments, and other elements are colored correctly.

### Supported Languages

Tap supports all languages available in Shiki, including:

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
| Markdown | `markdown`, `md` |

For the complete list, see the [Shiki languages documentation](https://shiki.style/languages).

::: tip Plain Text
Use `text` or `plaintext` for code blocks without highlighting:
```text
This is plain text with no syntax highlighting.
```
:::

## Line Highlighting

Draw attention to specific lines by adding line numbers in curly braces after the language:

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
| `{5}` | Highlight line 5 |
| `{1,3,5}` | Highlight lines 1, 3, and 5 |
| `{1-3}` | Highlight lines 1 through 3 |
| `{1-3,7,9-11}` | Combine ranges and individual lines |

### Example: Highlighting Changes

Use line highlighting to show what changed or what's important:

````markdown
```python {3-4}
def connect_to_database():
    config = load_config()
    connection = create_connection(config)  # new
    connection.verify()                      # new
    return connection
```
````

::: tip Best Practice
Highlight 1-3 lines at a time for clarity. Highlighting too many lines reduces the effectiveness.
:::

## Code Diffs

Show additions and removals using diff syntax:

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

Lines starting with `-` are styled as removals (typically red), and lines starting with `+` are styled as additions (typically green). Lines starting with a space are unchanged context.

### Alternative: Language-Specific Diffs

You can also combine diff markers with language highlighting:

````markdown
```javascript
// Before
- const data = fetchSync(url);
// After
+ const data = await fetch(url);
```
````

## Multi-Step Code Reveals

Reveal code incrementally by combining code blocks with the pause directive:

````markdown
# Building a Function

Let's build this step by step.

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

Each pause reveals the next version of the code, allowing you to walk through the implementation step by step.

### Progressive Line Highlighting

Combine multi-step reveals with line highlighting to focus attention:

````markdown
# Adding Error Handling

```javascript {1-3}
async function fetchUser(id) {
  const response = await api.get(`/users/${id}`);
  return response.data;
}
```

<!-- pause -->

Now let's add error handling:

```javascript {4-8}
async function fetchUser(id) {
  const response = await api.get(`/users/${id}`);
  return response.data;
  try {
    const response = await api.get(`/users/${id}`);
    return response.data;
  } catch (error) {
    console.error('Failed to fetch user:', error);
    throw error;
  }
}
```
````

## Font Size Configuration

Adjust code font size to fit more content or improve readability. Set the code font size in your presentation's frontmatter:

```yaml
---
title: Technical Deep Dive
codeFontSize: 14px
---
```

### Recommended Sizes

| Context | Size | Use Case |
|---------|------|----------|
| `18px` | Large | Few lines, audience far away |
| `16px` | Default | Standard presentations |
| `14px` | Medium | More code on screen |
| `12px` | Small | Dense code, close viewing |

::: tip Readability
Test your slides at the actual presentation distance. Code that looks fine on your laptop may be unreadable on a projector.
:::

## Code Block Styling

Themes control the overall appearance of code blocks, including:

- **Color scheme**: Light or dark syntax theme
- **Background**: Block background color
- **Border radius**: Rounded or sharp corners
- **Padding**: Space around the code

The `terminal` theme uses a dark, high-contrast code style, while `minimal` uses a lighter, more subtle appearance.

## Quick Reference

| Feature | Syntax | Example |
|---------|--------|---------|
| Language | ` ```language ` | ` ```python ` |
| Single line highlight | `{n}` | ` ```js {3} ` |
| Multiple lines | `{n,m,o}` | ` ```js {1,3,5} ` |
| Line range | `{n-m}` | ` ```js {2-5} ` |
| Combined | `{n-m,o}` | ` ```js {1-3,7} ` |
| Diff | ` ```diff ` | Shows +/- coloring |
| Font size | `codeFontSize` | In frontmatter |

## Next Steps

- Learn about [Live Code Execution](/guide/live-code-execution) to run code directly in your slides
- Explore [Themes](/guide/themes) to customize code appearance
- See [Writing Slides](/guide/writing-slides) for more markdown features
