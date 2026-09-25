import CryptoKit
import Foundation

/// What decides a thumbnail's pixels: tap's hash of the slide's content
/// (its text and its component bundle), the deck's theme signature, and
/// the snapshot width. Two slides with equal keys share one image.
public struct ThumbnailKey: Hashable, Sendable {
    public static let width = 320

    public let slideHash: String
    public let themeSignature: String
    public let width: Int

    public init(slideHash: String, themeSignature: String, width: Int = ThumbnailKey.width) {
        self.slideHash = slideHash
        self.themeSignature = themeSignature
        self.width = width
    }

    public var fileName: String {
        let digest = SHA256.hash(data: Data("\(slideHash)\n\(themeSignature)\n\(width)".utf8))
        return digest.map { String(format: "%02x", $0) }.joined() + ".png"
    }
}
