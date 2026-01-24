# Best Practices

Design tips for effective Tap presentations.

## Slide Design

### Keep Slides Focused
Each slide should convey one main idea. If you're cramming content, split it into multiple slides.

### Use Consistent Structure
Maintain a predictable rhythm:
1. Title slide
2. Agenda/overview
3. Content sections
4. Summary/call to action

### Leverage Layouts
Don't default to plain slides. Use layouts for visual variety:
- `two-column` for comparisons
- `quote` for testimonials
- `big-stat` for impact numbers
- `code-focus` for technical content

### Write Notes Liberally
Even if you know your content, speaker notes help you:
- Stay on track during nerves
- Remember specific data points
- Hand off presentations to colleagues

## Code Presentation

### Highlight Sparingly
Highlight 1-3 lines at a time. Too many highlights reduces effectiveness.

### Build Code Incrementally
Use `<!-- pause -->` to walk through code step by step:
````markdown
```javascript
function example() {
```

<!-- pause -->

```javascript
function example() {
  const result = calculate();
```

<!-- pause -->

```javascript
function example() {
  const result = calculate();
  return result;
}
```
````

### Use Appropriate Font Size
- `18px` - Large venue, few lines
- `16px` - Default
- `14px` - More code on screen
- `12px` - Dense code, close viewing

Test at actual presentation distance.

## Transitions & Animations

### Be Consistent
Use the same transition throughout sections. Mixed transitions feel chaotic.

### Use Zoom Sparingly
`zoom` is great for dramatic reveals but overuse is distracting. Reserve for key moments.

### Default to Fade
`fade` is smooth and professional. When in doubt, use it.

## Live Code Execution

### Test Before Presenting
Run through all slides with `tap dev` to verify queries work.

### Use Read-Only Credentials
Never connect to production with write access during demos.

### Have a Backup Plan
If database/network fails, have screenshots of expected output.

### Keep Queries Fast
Audiences lose attention during long-running operations. Optimize or add limits:
```sql
SELECT * FROM large_table LIMIT 10;
```

### Use SQLite for Portability
SQLite requires no external server—your presentation works anywhere.

## Theme Selection

| Context | Theme |
|---------|-------|
| Corporate/professional | `paper` |
| Executive/investor | `noir` |
| Startup/creative | `aurora` |
| Developer/technical | `phosphor` |
| Design/bold | `poster` |

## File Organization

```
my-presentation/
├── slides.md           # Main presentation
├── images/             # Image assets
│   ├── diagram.png
│   └── screenshot.jpg
├── data/               # Demo databases
│   └── demo.db
└── scripts/            # Demo scripts
    └── demo.sh
```

## Pre-Presentation Checklist

1. **Test live code** - Run `tap dev` and check all queries
2. **Check fonts** - Ensure fonts render at presentation distance
3. **Verify images** - All paths resolve correctly
4. **Test presenter mode** - Notes visible at `/presenter`
5. **Export backup PDF** - `tap pdf slides.md` just in case
6. **Check transitions** - Smooth on target hardware
7. **Prepare offline** - Build with `tap build` if network uncertain

## During Presentation

### Navigation
- Arrow keys or Space to advance
- `S` to open presenter view
- `F` for fullscreen
- `Esc` to exit fullscreen

### Presenter Mode
Open `/presenter` on your laptop while audience sees main view:
- Current slide
- Next slide preview
- Speaker notes
- Timer

### Recovery
If something breaks:
1. Advance to next slide
2. Use backup PDF if needed
3. Stay calm—audiences are forgiving
