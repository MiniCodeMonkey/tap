import AppKit

extension NSView {
    /// Puts `replacement` exactly where this view is: in the same
    /// superview, at the same place in the subview order, with the same
    /// frame and every constraint that names this view, then takes this
    /// view out. A view with no superview is left alone.
    func replaceInSuperview(with replacement: NSView) {
        guard let superview else { return }
        replacement.translatesAutoresizingMaskIntoConstraints = translatesAutoresizingMaskIntoConstraints
        replacement.autoresizingMask = autoresizingMask
        replacement.frame = frame
        superview.addSubview(replacement, positioned: .above, relativeTo: self)
        // Only plain constraints made by the app move: AppKit's own (the
        // autoresizing mask's, a view's content size) and a view's
        // constraints on its private subviews stay behind with it.
        let betweenViews = superview.constraints.filter { $0.firstItem === self || $0.secondItem === self }
        let onItself = constraints.filter { $0.firstItem === self && ($0.secondItem == nil || $0.secondItem === self) }
        let named = (betweenViews + onItself).filter { type(of: $0) == NSLayoutConstraint.self }
        let moved = named.map { constraint in
            let first: AnyObject? = constraint.firstItem === self ? replacement : constraint.firstItem
            let second: AnyObject? = constraint.secondItem === self ? replacement : constraint.secondItem
            let copy = NSLayoutConstraint(item: first as Any, attribute: constraint.firstAttribute,
                                          relatedBy: constraint.relation,
                                          toItem: second, attribute: constraint.secondAttribute,
                                          multiplier: constraint.multiplier, constant: constraint.constant)
            copy.priority = constraint.priority
            copy.identifier = constraint.identifier
            return copy
        }
        removeFromSuperview()
        NSLayoutConstraint.activate(moved)
    }
}
