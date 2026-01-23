---
title: Live Code Execution Demo
theme: terminal
author: Developer
date: 2026-01-24
aspectRatio: "16:9"
transition: slide
codeTheme: github-dark
fragments: true
drivers:
  shell:
    timeout: 30
  python:
    command: python3
    args: ["-c"]
    timeout: 10
---

# Live Code Execution

Tap can execute code directly in your presentations

---

## Shell Commands

Execute shell commands and see the output in real-time:

```bash {driver: shell}
echo "Hello from the shell!"
date
uname -a
```

Click **Run** or press `Ctrl+Enter` to execute.

---

<!--
layout: code-focus
-->

## File System Operations

```bash {driver: shell}
# List current directory
ls -la

# Show disk usage
df -h | head -5
```

---

## Environment Variables

Tap supports environment variables in your code:

```bash {driver: shell}
echo "Current user: $USER"
echo "Home directory: $HOME"
echo "Shell: $SHELL"
```

---

<!--
layout: two-column
-->

## Code on the Left

```bash {driver: shell}
# Process information
ps aux | head -10
```

|||

### Explanation

This command shows:
- Running processes
- CPU and memory usage
- Process IDs

Perfect for system monitoring demos!

---

## Python Examples

Use custom drivers for any language:

```python {driver: python}
import sys
print(f"Python version: {sys.version}")

# Simple calculation
result = sum(range(1, 101))
print(f"Sum of 1-100: {result}")
```

---

<!--
layout: code-focus
-->

## Data Processing with Python

```python {driver: python}
# Working with data
data = [
    {"name": "Alice", "score": 95},
    {"name": "Bob", "score": 87},
    {"name": "Charlie", "score": 92}
]

# Find average score
avg = sum(d["score"] for d in data) / len(data)
print(f"Average score: {avg:.1f}")

# Find top scorer
top = max(data, key=lambda x: x["score"])
print(f"Top scorer: {top['name']} ({top['score']})")
```

---

## Incremental Code Execution

<!-- pause -->

First, let's set up our environment:

```bash {driver: shell}
echo "Step 1: Environment check"
which python3 || echo "Python not found"
```

<!-- pause -->

Then run our analysis:

```bash {driver: shell}
echo "Step 2: Running analysis..."
echo "Complete!"
```

---

<!--
layout: section
-->

## Error Handling

What happens when things go wrong?

---

## Graceful Error Display

Tap handles errors gracefully:

```bash {driver: shell}
# This command will fail
cat /nonexistent/file 2>&1
echo "Exit code: $?"
```

Errors are displayed clearly without crashing the presentation.

---

## Timeout Protection

Long-running commands are automatically terminated:

```bash {driver: shell}
# This will be stopped by the timeout
echo "Starting..."
sleep 2
echo "Done!"
```

Configure timeouts in the frontmatter `drivers` section.

---

<!--
layout: big-stat
-->

## 30s

Default timeout for shell commands

---

## Best Practices

<!-- pause -->

1. **Keep commands fast** - Demos should be snappy

<!-- pause -->

2. **Handle errors gracefully** - Check exit codes

<!-- pause -->

3. **Use timeouts** - Prevent runaway processes

<!-- pause -->

4. **Test beforehand** - Know what output to expect

---

<!--
layout: title
-->

# Live Coding Made Easy

Impress your audience with real-time code execution
