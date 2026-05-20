// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "BrowserLabApp",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "BrowserLabApp", targets: ["BrowserLabApp"])
    ],
    targets: [
        .executableTarget(name: "BrowserLabApp"),
        .testTarget(name: "BrowserLabAppTests", dependencies: ["BrowserLabApp"])
    ]
)
