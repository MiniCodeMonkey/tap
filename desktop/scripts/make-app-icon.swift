#!/usr/bin/env swift
// Renders Tap's app icon and writes it into the asset catalog:
//   Tap/Assets.xcassets/AppIcon.appiconset      every size macOS asks for
//   Tap/Assets.xcassets/WelcomeIconBase.imageset the same icon without its caret,
//                                                which the welcome window blinks on top
// Run from desktop/: swift scripts/make-app-icon.swift
// The output is committed; rerun only to change the design.
//
// The design is a dark squircle lit from below by the aurora green, holding a
// translucent slide with a green caret. It is drawn on a 1024 point canvas
// with the Apple icon grid: an 824 point squircle inset 100 points, leaving
// room for the shadow. Every size is drawn from the vector description, not
// scaled from a bitmap.
import AppKit

let canvas: CGFloat = 1024
let shapeSize: CGFloat = 824
let shapeInset = (canvas - shapeSize) / 2
// One CSS pixel of the 96 pixel design, in canvas points.
let unit = shapeSize / 96

/// The squircle: a superellipse of exponent 5, close to Apple's continuous corner.
func squirclePath(in rect: CGRect) -> CGPath {
    let path = CGMutablePath()
    let exponent = 5.0
    let steps = 720
    for step in 0...steps {
        let angle = Double(step) / Double(steps) * 2 * .pi
        let cosine = cos(angle), sine = sin(angle)
        let x = CGFloat(copysign(pow(abs(cosine), 2 / exponent), cosine))
        let y = CGFloat(copysign(pow(abs(sine), 2 / exponent), sine))
        let point = CGPoint(x: rect.midX + x * rect.width / 2, y: rect.midY + y * rect.height / 2)
        if step == 0 { path.move(to: point) } else { path.addLine(to: point) }
    }
    path.closeSubpath()
    return path
}

func color(_ hex: UInt32, _ alpha: CGFloat = 1) -> CGColor {
    CGColor(srgbRed: CGFloat((hex >> 16) & 255) / 255, green: CGFloat((hex >> 8) & 255) / 255, blue: CGFloat(hex & 255) / 255, alpha: alpha)
}

func drawIcon(in context: CGContext, withCaret: Bool) {
    let space = CGColorSpace(name: CGColorSpace.sRGB)!
    let shape = CGRect(x: shapeInset, y: shapeInset, width: shapeSize, height: shapeSize)
    let path = squirclePath(in: shape)

    // The icon's own shadow, in the margin around the shape.
    context.saveGState()
    context.setShadow(offset: CGSize(width: 0, height: -14), blur: 26, color: color(0x000000, 0.34))
    context.addPath(path)
    context.setFillColor(color(0x0b0d12))
    context.fillPath()
    context.restoreGState()

    context.saveGState()
    context.addPath(path)
    context.clip()

    // radial-gradient(120% 90% at 50% 120%, #10b981 0%, #065f46 34%, #0b0d12 68%)
    let gradient = CGGradient(colorsSpace: space,
                              colors: [color(0x10b981), color(0x065f46), color(0x0b0d12), color(0x0b0d12)] as CFArray,
                              locations: [0, 0.34, 0.68, 1])!
    let radiusX = shapeSize * 1.2
    let radiusY = shapeSize * 0.9
    context.saveGState()
    context.translateBy(x: shape.midX, y: shape.minY - shapeSize * 0.2) // 120% down from the top is 20% below the bottom edge
    context.scaleBy(x: 1, y: radiusY / radiusX)
    context.drawRadialGradient(gradient, startCenter: .zero, startRadius: 0, endCenter: .zero, endRadius: radiusX, options: [.drawsAfterEndLocation])
    context.restoreGState()

    // A soft light from the top edge down to 45% of the height.
    let sheen = CGGradient(colorsSpace: space, colors: [color(0xffffff, 0.10), color(0xffffff, 0)] as CFArray, locations: [0, 1])!
    context.drawLinearGradient(sheen, start: CGPoint(x: 0, y: shape.maxY), end: CGPoint(x: 0, y: shape.maxY - shapeSize * 0.45), options: [])

    // The top highlight: the crescent of the shape that a copy of it moved down leaves uncovered.
    context.saveGState()
    let crescent = CGMutablePath()
    crescent.addPath(path)
    crescent.addPath(path, transform: CGAffineTransform(translationX: 0, y: -5))
    context.addPath(crescent)
    context.clip(using: .evenOdd)
    context.setFillColor(color(0xffffff, 0.16))
    context.fill(shape.insetBy(dx: -10, dy: -10))
    context.restoreGState()

    // The inner hairline.
    context.saveGState()
    context.addPath(path)
    context.setStrokeColor(color(0xffffff, 0.09))
    context.setLineWidth(6)
    context.strokePath()
    context.restoreGState()
    context.restoreGState()

    // The slide: 58 x 39 of the 96 point design, centered.
    let slide = CGRect(x: shape.midX - 29 * unit, y: shape.midY - 19.5 * unit, width: 58 * unit, height: 39 * unit)
    let slidePath = CGPath(roundedRect: slide, cornerWidth: 7 * unit, cornerHeight: 7 * unit, transform: nil)
    context.saveGState()
    context.setShadow(offset: CGSize(width: 0, height: -34), blur: 60, color: color(0x000000, 0.5))
    context.addPath(slidePath)
    context.setFillColor(color(0xffffff, 0.07))
    context.fillPath()
    context.restoreGState()
    context.saveGState()
    context.addPath(slidePath)
    context.setFillColor(color(0xffffff, 0.07))
    context.fillPath()
    context.addPath(slidePath)
    context.clip()
    context.addPath(slidePath)
    context.setStrokeColor(color(0xffffff, 0.16))
    context.setLineWidth(6)
    context.strokePath()
    context.restoreGState()

    guard withCaret else { return }
    let caret = CGRect(x: slide.minX + 11 * unit, y: slide.midY - 8.5 * unit, width: 4 * unit, height: 17 * unit)
    context.saveGState()
    context.setShadow(offset: .zero, blur: 12 * unit, color: color(0x7fd18c, 0.9))
    context.addPath(CGPath(roundedRect: caret, cornerWidth: 2 * unit, cornerHeight: 2 * unit, transform: nil))
    context.setFillColor(color(0x7fd18c))
    context.fillPath()
    context.restoreGState()
}

func pngData(pixels: Int, withCaret: Bool) -> Data {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels, pixelsHigh: pixels, bitsPerSample: 8, samplesPerPixel: 4,
                                  hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 32)!
    let context = NSGraphicsContext(bitmapImageRep: bitmap)!.cgContext
    context.interpolationQuality = .high
    context.scaleBy(x: CGFloat(pixels) / canvas, y: CGFloat(pixels) / canvas)
    drawIcon(in: context, withCaret: withCaret)
    return bitmap.representation(using: .png, properties: [:])!
}

let catalog = URL(fileURLWithPath: "Tap/Assets.xcassets", isDirectory: true)
let iconSet = catalog.appendingPathComponent("AppIcon.appiconset", isDirectory: true)
let baseSet = catalog.appendingPathComponent("WelcomeIconBase.imageset", isDirectory: true)
let manager = FileManager.default
for folder in [iconSet, baseSet] {
    try? manager.removeItem(at: folder)
    try manager.createDirectory(at: folder, withIntermediateDirectories: true)
}

let info = "\"info\" : { \"author\" : \"xcode\", \"version\" : 1 }"
try "{ \(info) }\n".write(to: catalog.appendingPathComponent("Contents.json"), atomically: true, encoding: .utf8)

var entries: [String] = []
for points in [16, 32, 128, 256, 512] {
    for scale in [1, 2] {
        let name = "icon_\(points)x\(points)\(scale == 2 ? "@2x" : "").png"
        try pngData(pixels: points * scale, withCaret: true).write(to: iconSet.appendingPathComponent(name))
        entries.append("    { \"filename\" : \"\(name)\", \"idiom\" : \"mac\", \"scale\" : \"\(scale)x\", \"size\" : \"\(points)x\(points)\" }")
    }
}
try "{\n  \"images\" : [\n\(entries.joined(separator: ",\n"))\n  ],\n  \(info)\n}\n".write(to: iconSet.appendingPathComponent("Contents.json"), atomically: true, encoding: .utf8)

try pngData(pixels: 512, withCaret: false).write(to: baseSet.appendingPathComponent("welcome-icon-base.png"))
try "{\n  \"images\" : [\n    { \"filename\" : \"welcome-icon-base.png\", \"idiom\" : \"universal\" }\n  ],\n  \(info)\n}\n".write(to: baseSet.appendingPathComponent("Contents.json"), atomically: true, encoding: .utf8)
print("wrote \(entries.count) icon sizes and the welcome base")
