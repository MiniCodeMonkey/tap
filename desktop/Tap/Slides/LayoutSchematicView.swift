import AppKit

/// Draws a layout's schematic: bars for headings and lines, blocks for
/// columns, code and media, a big bar for a statistic.
final class LayoutSchematicView: NSView {
    var elements: [LayoutSchematic.Element] = [] { didSet { needsDisplay = true } }

    override func draw(_ dirtyRect: NSRect) {
        NSColor.white.setFill()
        NSBezierPath(roundedRect: bounds, xRadius: 7, yRadius: 7).fill()
        let dark = NSColor.black.withAlphaComponent(0.38)
        let light = NSColor.black.withAlphaComponent(0.12)
        let heights: [CGFloat] = elements.map { element in
            switch element {
            case .heading: return 5
            case .line: return 3
            case .columns, .sidebar, .media: return 26
            case .code: return 30
            case .quote: return 14
            case .bigNumber: return 12
            }
        }
        let total = heights.reduce(0, +) + CGFloat(max(0, elements.count - 1)) * 4
        var y = (bounds.height + total) / 2
        for (element, height) in zip(elements, heights) {
            y -= height
            let width = bounds.width * 0.86
            let x = (bounds.width - width) / 2
            switch element {
            case .heading:
                dark.setFill(); NSRect(x: bounds.midX - width * 0.29, y: y, width: width * 0.58, height: height).fill()
            case .line:
                light.setFill(); NSRect(x: bounds.midX - width * 0.4, y: y, width: width * 0.8, height: height).fill()
            case .bigNumber:
                dark.setFill(); NSRect(x: bounds.midX - width * 0.22, y: y, width: width * 0.44, height: height).fill()
            case .columns(let count):
                let gap: CGFloat = 5
                let column = (width - gap * CGFloat(count - 1)) / CGFloat(count)
                for index in 0..<count {
                    light.setFill(); NSRect(x: x + CGFloat(index) * (column + gap), y: y, width: column, height: height).fill()
                }
            case .sidebar:
                light.setFill(); NSRect(x: x, y: y, width: width * 0.62, height: height).fill()
                dark.setFill(); NSRect(x: x + width * 0.68, y: y, width: width * 0.32, height: height).fill()
            case .media:
                light.setFill(); NSRect(x: x + width * 0.5, y: y, width: width * 0.5, height: height).fill()
                dark.setFill(); NSRect(x: x, y: y + height - 4, width: width * 0.42, height: 4).fill()
            case .code:
                dark.withAlphaComponent(0.2).setFill(); NSRect(x: x, y: y, width: width, height: height).fill()
            case .quote:
                dark.setFill(); NSRect(x: x, y: y, width: 3, height: height).fill()
                light.setFill(); NSRect(x: x + 8, y: y + height - 3, width: width * 0.7, height: 3).fill()
                NSRect(x: x + 8, y: y + 2, width: width * 0.5, height: 3).fill()
            }
            y -= 4
        }
    }
}
