import AppKit

/// Window > Tap Log: the output of each open deck's tap process, one tab per deck.
final class TapLogWindowController: NSWindowController {
    static let shared = TapLogWindowController()

    let picker = NSSegmentedControl(labels: [], trackingMode: .selectOne, target: nil, action: nil)
    let textView = NSTextView()
    private(set) var selectedLog: TapLog?
    private var logs: [TapLog] = []

    private init() {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 760, height: 460),
                              styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Tap Log"
        window.isReleasedWhenClosed = false
        window.center()
        super.init(window: window)

        textView.isEditable = false
        textView.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        textView.isVerticallyResizable = true
        textView.autoresizingMask = [.width]
        textView.textContainer?.widthTracksTextView = true
        let scrollView = NSScrollView()
        scrollView.documentView = textView
        scrollView.hasVerticalScroller = true
        picker.target = self
        picker.action = #selector(pickerChanged(_:))

        let root = NSView()
        for view in [picker, scrollView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            picker.topAnchor.constraint(equalTo: root.topAnchor, constant: 10),
            picker.centerXAnchor.constraint(equalTo: root.centerXAnchor),
            scrollView.topAnchor.constraint(equalTo: picker.bottomAnchor, constant: 10),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        window.contentView = root

        NotificationCenter.default.addObserver(self, selector: #selector(logDidAppend(_:)), name: TapLog.didAppendNotification, object: nil)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(log: TapLog?) {
        reload()
        if let log, let index = logs.firstIndex(where: { $0 === log }) { select(index) }
        showWindow(nil)
    }

    /// Lists the logs of the decks that are open now.
    func reload() {
        logs = NSDocumentController.shared.documents.flatMap { document -> [TapLog] in
            guard let controller = (document as? DeckDocument)?.sessionController else { return [] }
            let talk = controller.presentationIfCreated
            return [controller.session.log] + ((talk?.session?.log ?? (talk?.isActive == true ? talk?.lastTalkLog : talk?.lastTalkLogAfterFailure)).map { [$0] } ?? [])
        }
        picker.segmentCount = logs.count
        for (index, log) in logs.enumerated() {
            picker.setLabel(log.title, forSegment: index)
            picker.setWidth(0, forSegment: index)
        }
        let index = selectedLog.flatMap { selected in logs.firstIndex { $0 === selected } } ?? (logs.isEmpty ? nil : 0)
        if let index { select(index) } else { selectedLog = nil; textView.string = "" }
    }

    private func select(_ index: Int) {
        picker.selectedSegment = index
        selectedLog = logs[index]
        textView.string = logs[index].text
        textView.scrollToEndOfDocument(nil)
    }

    @objc private func pickerChanged(_ sender: NSSegmentedControl) {
        guard logs.indices.contains(sender.selectedSegment) else { return }
        select(sender.selectedSegment)
    }

    @objc private func logDidAppend(_ notification: Notification) {
        guard let log = notification.object as? TapLog, log === selectedLog, let line = log.lines.last else { return }
        textView.textStorage?.append(NSAttributedString(string: (textView.string.isEmpty ? "" : "\n") + line.formatted,
                                                        attributes: [.font: textView.font as Any, .foregroundColor: NSColor.labelColor]))
        textView.scrollToEndOfDocument(nil)
    }
}
