import Foundation
import Security

/// Where the Gemini key lives. The app reads it in two places only: to
/// put GEMINI_API_KEY into tap's environment, and to fill the Image
/// Generation pane's secure field. The value is never logged or shown.
public protocol GeminiKeyStore: AnyObject {
    func read() throws -> String?
    /// nil removes the key.
    func write(_ key: String?) throws
}

public enum KeychainError: Error, Equatable {
    case status(OSStatus)
}

/// The key as a generic password item in the login Keychain.
public final class KeychainGeminiKeyStore: GeminiKeyStore {
    public let service: String
    public let account: String

    public init(service: String = "io.geocod.tap.desktop", account: String = "GEMINI_API_KEY") {
        self.service = service
        self.account = account
    }

    private var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: account]
    }

    public func read() throws -> String? {
        var query = query
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw KeychainError.status(status) }
        guard let data = item as? Data else { return nil }
        return String(decoding: data, as: UTF8.self)
    }

    public func write(_ key: String?) throws {
        let deleted = SecItemDelete(query as CFDictionary)
        guard deleted == errSecSuccess || deleted == errSecItemNotFound else { throw KeychainError.status(deleted) }
        guard let key, !key.isEmpty else { return }
        var item = query
        item[kSecValueData as String] = Data(key.utf8)
        let added = SecItemAdd(item as CFDictionary, nil)
        guard added == errSecSuccess else { throw KeychainError.status(added) }
    }
}

/// A key store in memory: what the app uses under -TapDefaultsSuite (every
/// UI test launch) and every hosted test installs, so no test reads or
/// writes the person's Keychain. `writes` records each write, the value
/// included, for a test that stored a placeholder; nothing prints it.
public final class MemoryGeminiKeyStore: GeminiKeyStore {
    public var key: String?
    public private(set) var writes: [String?] = []

    public init(key: String? = nil) { self.key = key }

    public func read() throws -> String? { key }

    public func write(_ key: String?) throws {
        self.key = key
        writes.append(key)
    }
}

/// Which key tap gets: the login shell's GEMINI_API_KEY wins, then the
/// Keychain's, else none (tap's own no_api_key message then says so).
public enum GeminiKeySource: Equatable, Sendable {
    case shell
    case keychain
    case none

    public static func resolve(shellValue: String?, storedKey: String?) -> GeminiKeySource {
        if let shellValue, !shellValue.isEmpty { return .shell }
        if let storedKey, !storedKey.isEmpty { return .keychain }
        return .none
    }

    /// Adds the store's key to `environment` when the shell gave none. Called
    /// for the two image runs alone, on a copy of the environment: the key
    /// never enters a tap dev or tap present session, whose shell driver
    /// would hand it to any block on a slide.
    public static func apply(store: GeminiKeyStore, to environment: inout [String: String]) throws {
        if let shellValue = environment["GEMINI_API_KEY"], !shellValue.isEmpty { return }
        if let key = try store.read(), !key.isEmpty { environment["GEMINI_API_KEY"] = key }
    }
}
