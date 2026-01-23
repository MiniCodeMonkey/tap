---
title: Terminal Theme Showcase
theme: terminal
author: root
date: 2026-01-24
aspectRatio: "16:9"
transition: none
codeTheme: github-dark
drivers:
  shell:
    timeout: 10
---

# Terminal Theme

$ ./present --theme=hacker

---

## Design Philosophy

The terminal theme brings:

- **Hacker aesthetic** - Dark background, glowing text
- **Monospace everything** - Code-first design
- **CRT effects** - Retro scanlines and glow
- **Command-line feel** - Like presenting in vim

---

## Color Options

<!-- pause -->

$ echo "Default: Green on black"

<!-- pause -->

$ echo "Alternative: Amber on black"

<!-- pause -->

$ export GLOW_EFFECT=enabled

---

## Code Blocks

```bash
#!/bin/bash
# Terminal theme makes code shine

function present() {
    local topic="$1"
    echo "Presenting: $topic"

    while read -r slide; do
        render "$slide"
        wait_for_input
    done < slides.md
}

present "Terminal Theme"
```

---

## Live Execution

```bash {driver: shell}
echo "$ whoami"
whoami

echo ""
echo "$ date"
date

echo ""
echo "$ uptime"
uptime
```

---

<!--
layout: two-column
-->

## ASCII Art Welcome

```
 _____
|_   _|_ _ _ __
  | |/ _` | '_ \
  | | (_| | |_) |
  |_|\__,_| .__/
          |_|
```

|||

### Perfect For

- Developer conferences
- Technical deep-dives
- Security talks
- Hacking demos
- System administration
- DevOps presentations

---

<!--
layout: quote
-->

> /* There is no place like 127.0.0.1 */

Every Hacker Ever

---

<!--
layout: section
-->

## Visual Effects

$ cat /etc/effects

---

## CRT Effects

The terminal theme includes:

- **Scanlines** - Subtle horizontal lines
- **Text glow** - Phosphor-style glow
- **Flicker** - Slight CRT flicker
- **Matrix rain** - Optional background

All effects respect `prefers-reduced-motion`.

---

## System Information

```bash {driver: shell}
echo "=== System Info ==="
echo "Hostname: $(hostname)"
echo "OS: $(uname -s)"
echo "Kernel: $(uname -r)"
echo "Architecture: $(uname -m)"
echo "==================="
```

---

<!--
layout: big-stat
-->

## 1337

lines of pure aesthetics

---

## Terminal Tips

> Use > for blockquotes
> Like comments in config files

List items get $ prefix:
- First command
- Second command
- Third command

| Feature | Status |
|---------|--------|
| Glow | ON |
| Scanlines | ON |
| Flicker | SUBTLE |

---

## When to Use Terminal

- Hacker conferences (DEF CON, Black Hat)
- Developer meetups
- System administration talks
- Security presentations
- CLI tool demos
- Any talk where you want to look cool

---

<!--
layout: title
-->

# Terminal Theme

$ exit 0
