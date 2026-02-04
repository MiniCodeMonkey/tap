# Map Animations

Create animated map slides that fly between locations—perfect for visualizing journeys or showing geographic context.

## Basic Usage

````markdown
```map
start: 40.7128, -74.0060
end: 34.0522, -118.2437
```
````

The map loads at the start location. Press next to trigger the fly animation.

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `start` | Required | Starting coordinates (lat, lng) |
| `end` | Required | Ending coordinates (lat, lng) |
| `zoom` | `12` | Initial zoom level (1-20) |
| `endZoom` | same as zoom | Ending zoom level |
| `duration` | `3000` | Animation duration in milliseconds |
| `easing` | `ease-in-out` | Animation easing (`linear`, `ease-in`, `ease-out`, `ease-in-out`) |
| `pitch` | `0` | Camera tilt angle (0-85 degrees) |
| `bearing` | `0` | Camera rotation (0-360 degrees) |
| `markers` | `true` | Show start/end markers |
| `showPath` | `false` | Draw connecting line |

## Examples

### Cross-Country Flight

````markdown
```map
start: 40.7128, -74.0060
end: 34.0522, -118.2437
zoom: 5
duration: 8000
pitch: 45
showPath: true
```
````

### City Zoom

Zoom into a single location:

````markdown
```map
start: 51.5074, -0.1278
end: 51.5074, -0.1278
zoom: 10
endZoom: 16
duration: 4000
```
````

### Cinematic Flyover

Add pitch and bearing for 3D effect:

````markdown
```map
start: 37.7749, -122.4194
end: 37.7749, -122.4194
zoom: 12
endZoom: 15
pitch: 60
bearing: -30
```
````

## Navigation Behavior

Maps work like fragments:
1. Navigate to slide → map loads at start
2. Press next → animation flies to end
3. Press next again → advance to next slide

Going backward reverses this sequence.

## Full-Screen Maps

Use the `blank` layout:

````markdown
<!-- layout: blank -->

```map
start: 35.6762, 139.6503
end: 37.7749, -122.4194
zoom: 3
duration: 10000
```
````

## Zoom Level Guidelines

| Level | Scale |
|-------|-------|
| 4-8 | Country/continent |
| 10-14 | City |
| 15+ | Street-level |

## Notes

- Requires internet connection for map tiles
- Respects `prefers-reduced-motion` for accessibility
- Maps are non-interactive during presentation
- PDF export shows end position (no animation)
