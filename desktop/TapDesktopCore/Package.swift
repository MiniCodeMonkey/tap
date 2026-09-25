// swift-tools-version:5.10
import PackageDescription

let package = Package(
    name: "TapDesktopCore",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "TapDesktopCore", targets: ["TapDesktopCore"]),
    ],
    targets: [
        .target(name: "TapDesktopCore"),
        .testTarget(name: "TapDesktopCoreTests", dependencies: ["TapDesktopCore"]),
    ]
)
