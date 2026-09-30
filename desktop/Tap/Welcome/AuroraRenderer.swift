import Metal
import simd

/// What one aurora frame needs: the clock, how bright and how tall the lights are.
struct AuroraFrame: Equatable {
    var time: Float
    var intensity: Float
    var height: Float
}

/// Draws the aurora (Aurora.metal) into a texture: the lights into a
/// half-float scratch texture, then two Gaussian passes, the last one into
/// the target. The image is premultiplied, so it composites over any
/// background. The renderer owns no window and no view; the welcome
/// window's view and the tests both drive it.
final class AuroraRenderer {
    /// The four ramp colors of aurora.js, deep to light.
    static let colors: [SIMD4<Float>] = ["064e3b", "059669", "10b981", "6ee7b7"].map { hex in
        let value = UInt32(hex, radix: 16) ?? 0
        return SIMD4<Float>(Float((value >> 16) & 255) / 255, Float((value >> 8) & 255) / 255, Float(value & 255) / 255, 1)
    }
    /// The blur, in points of the window: CSS `blur(14px)` of the mockup.
    static let blurSigmaPoints: Float = 14

    private struct AuroraUniforms {
        var frame: SIMD4<Float>
        var shape: SIMD4<Float>
        var colors: (SIMD4<Float>, SIMD4<Float>, SIMD4<Float>, SIMD4<Float>)
    }
    private struct BlurUniforms { var step: SIMD4<Float> }

    private let device: MTLDevice
    private let scenePipeline: MTLRenderPipelineState
    private let horizontalPipeline: MTLRenderPipelineState
    private let finalPipeline: MTLRenderPipelineState
    private var sceneTexture: MTLTexture?
    private var blurTexture: MTLTexture?

    /// The pixel format `encode`'s target must have.
    let targetFormat: MTLPixelFormat

    /// nil when the app has no compiled shader library or a pipeline does not build.
    init?(device: MTLDevice, targetFormat: MTLPixelFormat = .bgra8Unorm, library: MTLLibrary? = nil) {
        guard let library = library ?? (try? device.makeDefaultLibrary(bundle: Bundle(for: AuroraRenderer.self))) ?? device.makeDefaultLibrary() else { return nil }
        func pipeline(_ fragment: String, _ format: MTLPixelFormat) -> MTLRenderPipelineState? {
            let descriptor = MTLRenderPipelineDescriptor()
            descriptor.vertexFunction = library.makeFunction(name: "auroraVertex")
            descriptor.fragmentFunction = library.makeFunction(name: fragment)
            descriptor.colorAttachments[0].pixelFormat = format
            guard descriptor.vertexFunction != nil, descriptor.fragmentFunction != nil else { return nil }
            return try? device.makeRenderPipelineState(descriptor: descriptor)
        }
        guard let scene = pipeline("auroraFragment", .rgba16Float),
              let horizontal = pipeline("auroraBlurFragment", .rgba16Float),
              let final = pipeline("auroraFinalBlurFragment", targetFormat) else { return nil }
        self.device = device
        self.targetFormat = targetFormat
        scenePipeline = scene
        horizontalPipeline = horizontal
        finalPipeline = final
    }

    private func scratchTexture(width: Int, height: Int) -> MTLTexture? {
        let descriptor = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .rgba16Float, width: width, height: height, mipmapped: false)
        descriptor.usage = [.renderTarget, .shaderRead]
        descriptor.storageMode = .private
        return device.makeTexture(descriptor: descriptor)
    }

    /// Encodes one frame into `target`. `pointsPerTexel` is how many points of
    /// the window one texel of the target covers, which sets the blur's width.
    func encode(_ frame: AuroraFrame, into target: MTLTexture, pointsPerTexel: Float, commandBuffer: MTLCommandBuffer) {
        let width = target.width, height = target.height
        if sceneTexture?.width != width || sceneTexture?.height != height {
            sceneTexture = scratchTexture(width: width, height: height)
            blurTexture = scratchTexture(width: width, height: height)
        }
        guard let sceneTexture, let blurTexture else { return }
        let sigma = Self.blurSigmaPoints / max(pointsPerTexel, 0.001)

        var uniforms = AuroraUniforms(frame: SIMD4(Float(width), Float(height), frame.time, frame.intensity),
                                      shape: SIMD4(frame.height, 0, 0, 0),
                                      colors: (Self.colors[0], Self.colors[1], Self.colors[2], Self.colors[3]))
        pass(commandBuffer, target: sceneTexture, pipeline: scenePipeline) { encoder in
            encoder.setFragmentBytes(&uniforms, length: MemoryLayout<AuroraUniforms>.stride, index: 0)
        }
        var horizontal = BlurUniforms(step: SIMD4(1 / Float(width), 0, sigma, 0))
        pass(commandBuffer, target: blurTexture, pipeline: horizontalPipeline) { encoder in
            encoder.setFragmentTexture(sceneTexture, index: 0)
            encoder.setFragmentBytes(&horizontal, length: MemoryLayout<BlurUniforms>.stride, index: 0)
        }
        var vertical = BlurUniforms(step: SIMD4(0, 1 / Float(height), sigma, 0))
        pass(commandBuffer, target: target, pipeline: finalPipeline) { encoder in
            encoder.setFragmentTexture(blurTexture, index: 0)
            encoder.setFragmentBytes(&vertical, length: MemoryLayout<BlurUniforms>.stride, index: 0)
        }
    }

    private func pass(_ commandBuffer: MTLCommandBuffer, target: MTLTexture, pipeline: MTLRenderPipelineState, configure: (MTLRenderCommandEncoder) -> Void) {
        let descriptor = MTLRenderPassDescriptor()
        descriptor.colorAttachments[0].texture = target
        descriptor.colorAttachments[0].loadAction = .dontCare
        descriptor.colorAttachments[0].storeAction = .store
        guard let encoder = commandBuffer.makeRenderCommandEncoder(descriptor: descriptor) else { return }
        encoder.setRenderPipelineState(pipeline)
        configure(encoder)
        encoder.drawPrimitives(type: .triangleStrip, vertexStart: 0, vertexCount: 4)
        encoder.endEncoding()
    }
}
