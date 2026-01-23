---
title: Presenter Mode
---

# Presenter Mode

Tap's presenter mode gives you a powerful dual-window setup: your audience sees the presentation while you see speaker notes, a timer, and a preview of upcoming slides.

## How Presenter Mode Works

Presenter mode creates two synchronized views of your presentation:

1. **Audience View** (`/`) - The full-screen presentation your audience sees
2. **Presenter View** (`/presenter`) - Your private control panel with notes, timer, and navigation

Both views stay perfectly in sync. Advance the slide in either window and the other follows automatically.

### Starting Presenter Mode

```bash
# Start the dev server
tap dev presentation.md

# Open presenter view in a new tab or window
# Navigate to http://localhost:3000/presenter
```

::: tip Keyboard Shortcut
Press **S** during the presentation to open the presenter view in a new window.
:::

## Presenter View Features

The presenter view includes everything you need to deliver a polished presentation:

### Speaker Notes

Your notes appear prominently in the presenter view. Add notes to any slide using the `notes` directive:

```markdown
# Quarterly Results

Revenue grew 23% year over year.

<!--
notes: |
  - Mention the new product launch in Q2
  - Highlight the APAC expansion
  - Transition to next quarter goals
-->
```

Notes support full markdown formatting, so you can use bullet points, bold text, and even code snippets.

### Timer

The presenter view includes a timer that starts when you begin presenting:

- **Elapsed time** - How long you've been presenting
- **Current time** - The current wall clock time

::: tip Reset Timer
Press **R** in presenter view to reset the timer to zero.
:::

### Next Slide Preview

A preview of the upcoming slide appears in the presenter view, helping you:

- Prepare smooth transitions
- Remember what's coming next
- Avoid surprises during your talk

### Current Slide Preview

Your current slide is displayed in the presenter view so you can see exactly what your audience sees without turning around.

## Cross-Device Presenter Mode

One of Tap's most powerful features is the ability to control your presentation from a separate device.

### Using an iPad or Phone as a Controller

1. Start `tap dev` on your laptop
2. Connect your iPad/phone to the same network
3. Navigate to `http://<your-laptop-ip>:3000/presenter` on your device
4. Your device becomes a wireless presentation remote

This setup lets you:

- Walk around freely while presenting
- See your notes without looking at your laptop
- Control slides with touch gestures

### QR Code for Easy Access

When you start the dev server, Tap displays a QR code in the terminal:

```bash
tap dev presentation.md

# Output includes:
#   Local:   http://localhost:3000
#   Network: http://192.168.1.100:3000
#   [QR CODE]
```

Scan the QR code with your phone or tablet to instantly open the presentation. Navigate to `/presenter` for the presenter view.

## Password Protection

For sensitive presentations, you can protect the presenter view with a password:

```yaml
---
title: Confidential Results
presenterPassword: secret123
---
```

When password protection is enabled:

- The audience view (`/`) remains publicly accessible
- The presenter view (`/presenter`) requires the password
- Notes and upcoming slides stay private

::: warning
The password is stored in plain text in your markdown file. Don't commit sensitive passwords to version control.
:::

## Presenter Mode Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| **S** | Open presenter view (from audience view) |
| **Right** / **Space** | Next slide |
| **Left** | Previous slide |
| **R** | Reset timer |
| **O** | Toggle slide overview |
| **Esc** | Exit overview / fullscreen |
| **F** | Toggle fullscreen |
| **B** | Black screen (pause) |

## Best Practices

### Before Your Talk

1. **Test presenter mode** on the actual display setup
2. **Check network connectivity** if using cross-device control
3. **Set a password** for confidential content
4. **Write notes** for complex or data-heavy slides

### During Your Talk

1. **Use the timer** to pace yourself
2. **Glance at the next slide preview** before transitions
3. **Keep notes concise** - bullet points work better than paragraphs

### Tip: Dual Monitor Setup

The ideal setup uses two displays:

1. **External display** - Shows audience view in fullscreen (press **F**)
2. **Laptop screen** - Shows presenter view with notes

This mirrors the classic conference room setup while giving you modern features like cross-device sync.

## Next Steps

- [Keyboard Shortcuts](/reference/keyboard-shortcuts) - Complete shortcut reference
- [Writing Slides](/guide/writing-slides) - Learn about speaker notes syntax
- [Building & Export](/guide/building-export) - Export with or without notes
