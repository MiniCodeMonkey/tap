import AppKit
import MetalKit
import TapDesktopCore

/// The tap.sh northern lights behind the welcome window, drawn by Metal at
/// half the window's size in points and blurred, over the window's own
/// background. It costs almost nothing: a tiny texture, 30 frames a second,
/// and no frames at all while the window is hidden, covered or minimized,
/// the app is hidden, or Reduce Motion is on (one still frame then).
final class AuroraView: NSView, MTKViewDelegate {
    static let framesPerSecond = 30
    /// The clock runs slower than wall time, as on the site.
    private static let clockRate = 0.65
    /// The shader's noise offset that the site's clock starts at.
    private static let clockOrigin = 40.0

    private let metalView: MTKView?
    private let renderer: AuroraRenderer?
    private let commandQueue: MTLCommandQueue?
    private let fallbackLayer = CAGradientLayer()

    private var clock = 0.0
    private var lastFrameTime: CFTimeInterval?
    private var openedAt = CACurrentMediaTime()
    private var boost = 1.0
    private var isDragBoosted = false
    private var observers: [NSObjectProtocol] = []
    private var workspaceObserver: NSObjectProtocol?

    /// Whether the window can be seen; the window's occlusion state feeds it.
    private(set) var isWindowVisible = false
    private(set) var isAppHidden = false
    /// True while frames are being drawn continuously.
    private(set) var isAnimating = false
    private(set) var reduceMotion = false
    /// Frames drawn since the view was created.
    private(set) var framesDrawn = 0

    /// True when Metal built the renderer; false draws the still fallback gradient.
    var rendersWithMetal: Bool { renderer != nil }

    override init(frame frameRect: NSRect) {
        let device = MTLCreateSystemDefaultDevice()
        let renderer = device.flatMap { AuroraRenderer(device: $0) }
        self.renderer = renderer
        commandQueue = device.flatMap { renderer == nil ? nil : $0.makeCommandQueue() }
        if let device, renderer != nil {
            let view = MTKView(frame: .zero, device: device)
            view.colorPixelFormat = .bgra8Unorm
            view.framebufferOnly = true
            view.autoResizeDrawable = false
            view.clearColor = MTLClearColor(red: 0, green: 0, blue: 0, alpha: 0)
            view.preferredFramesPerSecond = Self.framesPerSecond
            view.enableSetNeedsDisplay = false
            view.isPaused = true
            metalView = view
        } else {
            metalView = nil
        }
        super.init(frame: frameRect)
        wantsLayer = true
        setAccessibilityElement(false)
        if let metalView {
            (metalView.layer as? CAMetalLayer)?.isOpaque = false
            metalView.delegate = self
            metalView.autoresizingMask = [.width, .height]
            metalView.frame = bounds
            addSubview(metalView)
        } else {
            fallbackLayer.type = .radial
            fallbackLayer.startPoint = CGPoint(x: 0.5, y: 0)
            fallbackLayer.endPoint = CGPoint(x: 1, y: 0.72)
            fallbackLayer.locations = [0, 0.45, 1]
            layer?.addSublayer(fallbackLayer)
        }
        reduceMotion = WelcomeMotion.reduceMotion()
        workspaceObserver = NSWorkspace.shared.notificationCenter.addObserver(forName: NSWorkspace.accessibilityDisplayOptionsDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.motionSettingChanged() }
        }
        let center = NotificationCenter.default
        observers = [
            center.addObserver(forName: NSApplication.didHideNotification, object: nil, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.appVisibilityChanged(hidden: true) }
            },
            center.addObserver(forName: NSApplication.didUnhideNotification, object: nil, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.appVisibilityChanged(hidden: false) }
            },
        ]
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    deinit {
        (observers + windowObservers).forEach(NotificationCenter.default.removeObserver)
        workspaceObserver.map(NSWorkspace.shared.notificationCenter.removeObserver)
    }

    override func hitTest(_ point: NSPoint) -> NSView? { nil }

    // MARK: Run state

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        for observer in windowObservers { NotificationCenter.default.removeObserver(observer) }
        windowObservers = []
        guard let window else {
            windowVisibilityChanged(false)
            return
        }
        for name in [NSWindow.didChangeOcclusionStateNotification, NSWindow.didMiniaturizeNotification, NSWindow.didDeminiaturizeNotification, NSWindow.willCloseNotification] {
            windowObservers.append(NotificationCenter.default.addObserver(forName: name, object: window, queue: .main) { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let self, let window = self.window else { return }
                    self.windowVisibilityChanged(notification.name != NSWindow.willCloseNotification && Self.canBeSeen(window))
                }
            })
        }
        windowVisibilityChanged(Self.canBeSeen(window))
    }

    private var windowObservers: [NSObjectProtocol] = []

    private static func canBeSeen(_ window: NSWindow) -> Bool {
        window.isVisible && !window.isMiniaturized && window.occlusionState.contains(.visible)
    }

    /// The window became visible or stopped being; the view draws only while it is.
    func windowVisibilityChanged(_ isVisible: Bool) {
        isWindowVisible = isVisible
        refreshRunState()
    }

    private func appVisibilityChanged(hidden: Bool) {
        isAppHidden = hidden
        refreshRunState()
    }

    private func motionSettingChanged() {
        reduceMotion = WelcomeMotion.reduceMotion()
        refreshRunState()
    }

    /// Re-reads Reduce Motion, then starts or stops the frame loop.
    func refreshRunState() {
        reduceMotion = WelcomeMotion.reduceMotion()
        let wantsFrames = isWindowVisible && !isAppHidden && !reduceMotion
        if wantsFrames != isAnimating {
            isAnimating = wantsFrames
            lastFrameTime = nil
            metalView?.enableSetNeedsDisplay = !wantsFrames
            metalView?.isPaused = !wantsFrames
        }
        if !wantsFrames, isWindowVisible, !isAppHidden { drawStillFrame() }
    }

    /// The lights start rising again, as they do each time the window opens.
    func restartOpening() {
        openedAt = CACurrentMediaTime()
        lastFrameTime = nil
        if !isAnimating { drawStillFrame() }
    }

    /// The lights brighten while a file is dragged over the window. With
    /// Reduce Motion there is no easing: the still frame changes at once.
    func setDragBoosted(_ boosted: Bool) {
        isDragBoosted = boosted
        if !isAnimating {
            boost = boosted ? AuroraTimeline.dragBoost : 1
            drawStillFrame()
        }
    }

    // MARK: Drawing

    private var isDark: Bool { effectiveAppearance.isDarkAppearance }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        updateFallback()
        if !isAnimating { drawStillFrame() }
    }

    override func layout() {
        super.layout()
        let scale = window?.backingScaleFactor ?? 2
        metalView?.drawableSize = CGSize(width: max(2, (bounds.width / 2).rounded()), height: max(2, (bounds.height / 2).rounded()))
        metalView?.layer?.contentsScale = scale
        fallbackLayer.frame = bounds
        updateFallback()
        if !isAnimating { drawStillFrame() }
    }

    private func updateFallback() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            fallbackLayer.colors = [NSColor(hex: 0x059669, alpha: isDark ? 0.5 : 0.35), NSColor(hex: 0x10b981, alpha: isDark ? 0.2 : 0.12), NSColor(hex: 0x10b981, alpha: 0)].map(\.cgColor)
        }
    }

    private func drawStillFrame() {
        guard let metalView, renderer != nil, isWindowVisible || window == nil else { return }
        metalView.draw()
    }

    /// The opacity the layer should have now: the fade-in after the window opened.
    private func currentOpacity(now: CFTimeInterval) -> Float {
        Float(AuroraTimeline.opacity(elapsed: now - openedAt, reduceMotion: reduceMotion))
    }

    func mtkView(_ view: MTKView, drawableSizeWillChange size: CGSize) {}

    func draw(in view: MTKView) {
        guard let renderer, let commandQueue, let drawable = view.currentDrawable, let commandBuffer = commandQueue.makeCommandBuffer() else { return }
        let now = CACurrentMediaTime()
        let delta = min(now - (lastFrameTime ?? now), 0.1)
        lastFrameTime = now
        if isAnimating {
            clock += delta * Self.clockRate
            boost = AuroraTimeline.boost(current: boost, dragging: isDragBoosted, deltaSeconds: delta)
        }
        let resting = AuroraTimeline.restingHeight(isDark: isDark)
        let frame = AuroraFrame(
            time: Float(clock + Self.clockOrigin),
            intensity: Float(AuroraTimeline.restingIntensity(isDark: isDark) * boost),
            height: Float(AuroraTimeline.height(elapsed: now - openedAt, resting: resting, reduceMotion: reduceMotion)))
        renderer.encode(frame, into: drawable.texture, pointsPerTexel: Float(bounds.width / CGFloat(max(drawable.texture.width, 1))), commandBuffer: commandBuffer)
        commandBuffer.present(drawable)
        commandBuffer.commit()
        framesDrawn += 1
        let opacity = currentOpacity(now: now)
        if metalView?.layer?.opacity != opacity { metalView?.layer?.opacity = opacity }
        // The rise and fade take a few seconds; a still frame drawn early would freeze mid-way, so it draws again until they end.
        if !isAnimating, !reduceMotion, now - openedAt < AuroraTimeline.riseDuration {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0 / 30) { [weak self] in
                MainActor.assumeIsolated { if self?.isAnimating == false { self?.drawStillFrame() } }
            }
        }
    }
}
