---
title: Map Animations
---

# Map Animations

Tap supports animated map slides that fly between two locations, perfect for visualizing journeys, showing geographic context, or adding visual interest to travel-related presentations.

## Basic Usage

Add a map code block to your slide:

```markdown
```map
start: 40.7128, -74.0060
end: 34.0522, -118.2437
```
```

The map appears at the start location. Press the next key to trigger the fly animation.

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `start` | Required | Starting coordinates (lat, lng) |
| `end` | Required | Ending coordinates (lat, lng) |
| `zoom` | `12` | Initial zoom level (1-20) |
| `endZoom` | same as zoom | Ending zoom level |
| `duration` | `3000` | Animation duration in milliseconds |
| `easing` | `ease-in-out` | Animation easing function |
| `pitch` | `0` | Camera tilt angle (0-85 degrees) |
| `bearing` | `0` | Camera rotation (0-360 degrees) |
| `markers` | `true` | Show start/end markers |
| `showPath` | `false` | Draw connecting line |
| `style` | `geocodio` | Map style URL or `geocodio` for default |

### Easing Functions

- `linear` - Constant speed throughout
- `ease-in` - Start slow, end fast
- `ease-out` - Start fast, end slow
- `ease-in-out` - Slow at both ends (default)

## Examples

### Cross-Country Flight

```markdown
```map
start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 5
duration: 8000
pitch: 45
markers: true
showPath: true
```
```

### City Zoom

Zoom into a single location by using the same coordinates for start and end:

```markdown
```map
start: 51.5074, -0.1278
end: 51.5074, -0.1278
zoom: 10
endZoom: 16
duration: 4000
```
```

### European Tour

```markdown
```map
start: 48.8566, 2.3522
end: 41.9028, 12.4964
zoom: 5
duration: 6000
bearing: 45
```
```

### Cinematic Flyover

Add pitch and bearing for a 3D cinematic effect:

```markdown
```map
start: 37.7749, -122.4194
end: 37.7749, -122.4194
zoom: 12
endZoom: 15
duration: 5000
pitch: 60
bearing: -30
```
```

## Animation Behavior

Map animations work like fragments:

1. Navigate to the slide → map loads at start position
2. Press next (Space/→) → animation flies to end position
3. Press next again → advance to next slide

### Going Backward

When navigating backward:
- From next slide → returns to map at end position
- Press previous → map resets to start position
- Press previous again → go to previous slide

## Tips

### Keep It Readable

- Use zoom levels 4-8 for country/continent scale
- Use zoom levels 10-14 for city scale
- Use zoom levels 15+ for street-level detail

### Combine with Layouts

Use the `blank` layout for full-screen maps:

```markdown
---

<!-- layout: blank -->

```map
start: 35.6762, 139.6503
end: 37.7749, -122.4194
zoom: 3
duration: 10000
```

---
```

### Network Requirements

Map slides require an internet connection to load map tiles. For offline presentations, consider using the PDF export or taking screenshots.

### Performance

- Maps use WebGL for rendering, which may not work in all browsers
- Each map slide creates a WebGL context; having many map slides may impact performance
- The map is destroyed when navigating away to free resources

## Accessibility

Map animations automatically respect the user's motion preferences:

- With `prefers-reduced-motion: reduce`, animations jump instantly to the end position
- Keyboard navigation works as expected
- The map is non-interactive during presentation (prevents accidental panning)

## PDF Export

When exporting to PDF:
- Maps automatically show the end position (no animation)
- The exporter waits for map tiles to load before capturing
- Attribution is hidden in print mode

## Quick Reference

| Feature | Syntax |
|---------|--------|
| Basic map | ` ```map ... ``` ` |
| Set start | `start: lat, lng` |
| Set end | `end: lat, lng` |
| Zoom level | `zoom: 12` |
| End zoom | `endZoom: 16` |
| Duration | `duration: 5000` |
| Show path | `showPath: true` |
| Hide markers | `markers: false` |
| Tilt camera | `pitch: 45` |
| Rotate | `bearing: 90` |
| Easing | `easing: ease-out` |

## Troubleshooting

### Map Not Loading

- Check your internet connection
- Ensure WebGL is enabled in your browser
- Try refreshing the page

### Animation Not Triggering

- Make sure you're pressing the navigation key (Space, →, etc.)
- Check that the slide is active
- Verify the map configuration syntax is correct

### Tiles Not Loading

- The default map style uses Carto's Positron tiles
- Some corporate networks may block map tile servers
- Try using a custom `style` URL if needed

## Next Steps

- Learn about [Layouts](/guide/layouts) for full-screen maps
- Explore [Animations & Transitions](/guide/animations-transitions) for slide effects
- See [Building & Export](/guide/building-export) for PDF considerations
