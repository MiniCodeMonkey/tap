import XCTest
@testable import TapDesktopCore

final class DeckSchemaTests: XCTestCase {
    let json = #"""
    {"ok":true,"keys":[
      {"name":"title","type":"string","default":null,"description":"The deck's title."},
      {"name":"theme","type":"string","default":"base","values":["base","midnight"],"description":"The built-in theme."},
      {"name":"slideNumbers","type":"boolean","default":true,"description":"Numbers."},
      {"name":"themeColors","type":"object","default":null,"description":"Colors.","keys":[{"name":"background","type":"string","default":null,"description":"bg"}]},
      {"name":"drivers","type":"map","default":null,"description":"Drivers.","keys":[
        {"name":"command","type":"string","default":null,"description":"cmd"},
        {"name":"args","type":"list","default":null,"description":"args"},
        {"name":"timeout","type":"integer","default":30,"description":"Seconds."},
        {"name":"connections","type":"map","default":null,"description":"conns","keys":[{"name":"password","type":"string","default":null,"description":"pw"}]}]}
    ]}
    """#

    func testDecodesTheSchema() throws {
        let keys = try DeckSchema.decode(Data(json.utf8))
        XCTAssertEqual(keys.map(\.name), ["title", "theme", "slideNumbers", "themeColors", "drivers"])
        XCTAssertEqual(keys[0], SchemaKey(name: "title", type: "string", defaultValue: nil, values: [], description: "The deck's title.", keys: []))
        XCTAssertEqual(keys[1].defaultValue, "base")
        XCTAssertEqual(keys[1].values, ["base", "midnight"])
        XCTAssertEqual(keys[2].defaultValue, "true")
        XCTAssertEqual(keys[3].keys.map(\.name), ["background"])
        XCTAssertEqual(keys[4].keys[2].defaultValue, "30")
        XCTAssertEqual(keys[4].keys[3].keys.map(\.name), ["password"])
        XCTAssertTrue(keys[0].isScalar)
        XCTAssertTrue(keys[4].keys[1].isScalar, "a list is edited as one line")
        XCTAssertFalse(keys[3].isScalar)
        XCTAssertFalse(keys[4].isScalar)
    }

    func testAListOrObjectDefaultDecodesAsItsFlowText() throws {
        let json = #"{"ok":true,"keys":[{"name":"a","type":"list","default":["x","y: z",null,2]},{"name":"b","type":"object","default":{"k":"v","j":[true]}},{"name":"c","type":"string","default":"plain"}]}"#
        let keys = try DeckSchema.decode(Data(json.utf8))
        XCTAssertEqual(keys.map(\.defaultValue), ["[x, \"y: z\", null, 2]", "{j: [true], k: v}", "plain"],
                       "a default that is not a scalar never fails the whole schema")
    }

    func testLabelsReadAsWords() {
        XCTAssertEqual(SchemaKey(name: "aspectRatio", type: "string").label, "Aspect ratio")
        XCTAssertEqual(SchemaKey(name: "title", type: "string").label, "Title")
        XCTAssertEqual(SchemaKey(name: "themeColors", type: "object").label, "Theme colors")
        XCTAssertEqual(SchemaKey(name: "codeBg", type: "string").label, "Code bg")
    }

    func testFindsAKeyByPathThroughMaps() throws {
        let keys = try DeckSchema.decode(Data(json.utf8))
        XCTAssertEqual(DeckSchema.key(at: ["theme"], in: keys)?.name, "theme")
        XCTAssertEqual(DeckSchema.key(at: ["themeColors", "background"], in: keys)?.name, "background")
        XCTAssertEqual(DeckSchema.key(at: ["drivers", "sqlite", "timeout"], in: keys)?.name, "timeout", "a map's entry name is any name")
        XCTAssertEqual(DeckSchema.key(at: ["drivers", "sqlite", "connections", "incident", "password"], in: keys)?.name, "password")
        XCTAssertNil(DeckSchema.key(at: ["drivers", "sqlite", "nope"], in: keys))
        XCTAssertNil(DeckSchema.key(at: [], in: keys))
    }

    func testRefusesAnErrorAnswer() {
        XCTAssertThrowsError(try DeckSchema.decode(Data(#"{"ok":false,"error":{"code":"internal","message":"no"}}"#.utf8)))
    }
}
