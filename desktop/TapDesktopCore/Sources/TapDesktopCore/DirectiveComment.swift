import Foundation

public struct DirectiveComment: Equatable, Sendable {
    public let range: NSRange
    public let body: String
    public static func leading(in slideText: String) -> DirectiveComment? { nil }
    public static func rewrite(slideText: String, setting key: String, to value: String?) -> String { slideText }
}
