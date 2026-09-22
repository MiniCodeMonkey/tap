import CoreGraphics

/// The app owns the divider between the editor and the right pane. It sits
/// in the middle after every layout change until the user drags it.
public struct DividerPolicy: Equatable, Sendable {
    public private(set) var userMovedDivider = false

    public init() {}

    public mutating func userDragged() { userMovedDivider = true }
    public mutating func reset() { userMovedDivider = false }

    /// The divider position that splits `totalWidth` in half, or nil once the user moved it.
    public func balancedPosition(totalWidth: CGFloat, dividerThickness: CGFloat) -> CGFloat? {
        guard !userMovedDivider else { return nil }
        return ((totalWidth - dividerThickness) / 2).rounded(.down)
    }
}
