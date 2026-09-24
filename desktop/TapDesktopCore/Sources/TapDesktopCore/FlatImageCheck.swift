import AppKit

/// Whether a snapshot is one flat colour, which is what an unpainted page
/// gives. The image is drawn down to a small bitmap and every pixel of
/// that bitmap is read, so a thin stroke anywhere still counts as content
/// (it shades the block it lands in), unlike a grid of sample points that
/// can fall between strokes.
public enum FlatImageCheck {
    static let sampleSize = NSSize(width: 64, height: 36)

    public static func isFlat(_ image: NSImage) -> Bool {
        guard image.size.width > 0, image.size.height > 0,
              let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(sampleSize.width), pixelsHigh: Int(sampleSize.height),
                                            bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                            colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return true }
        NSGraphicsContext.saveGraphicsState()
        defer { NSGraphicsContext.restoreGraphicsState() }
        guard let context = NSGraphicsContext(bitmapImageRep: bitmap) else { return true }
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        image.draw(in: NSRect(origin: .zero, size: sampleSize), from: .zero, operation: .copy, fraction: 1)
        context.flushGraphics()
        var minimum = [255, 255, 255]
        var maximum = [0, 0, 0]
        for y in 0..<bitmap.pixelsHigh {
            for x in 0..<bitmap.pixelsWide {
                guard let colour = bitmap.colorAt(x: x, y: y) else { continue }
                let channels = [Int(colour.redComponent * 255), Int(colour.greenComponent * 255), Int(colour.blueComponent * 255)]
                for index in 0..<3 {
                    minimum[index] = min(minimum[index], channels[index])
                    maximum[index] = max(maximum[index], channels[index])
                }
            }
        }
        return (0..<3).allSatisfy { maximum[$0] - minimum[$0] <= 6 }
    }
}
