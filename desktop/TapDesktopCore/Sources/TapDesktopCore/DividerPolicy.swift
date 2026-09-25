import CoreGraphics

/// The app owns the divider between the editor and the right pane. It sits
/// in the middle after every layout change until the user drags it.
public struct DividerPolicy: Equatable, Sendable {
    public private(set) var userMovedDivider = false

    public init() {}

    public mutating func userDragged() { userMovedDivider = true }
    public mutating func reset() { userMovedDivider = false }

    /// The position of the divider between the editor and the right pane
    /// that splits them evenly, measured from the split view's left edge,
    /// or nil once the user moved it. `leadingWidth` is the width of a
    /// pinned sidebar in front of the editor, 0 when there is none; its own
    /// divider takes `dividerThickness` too.
    public func balancedPosition(totalWidth: CGFloat, dividerThickness: CGFloat, leadingWidth: CGFloat = 0) -> CGFloat? {
        guard !userMovedDivider else { return nil }
        let leading = leadingWidth > 0 ? leadingWidth + dividerThickness : 0
        let remaining = totalWidth - leading - dividerThickness
        return leading + (remaining / 2).rounded(.down)
    }
}
