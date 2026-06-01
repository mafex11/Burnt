// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Burnt",
    platforms: [.macOS(.v14)],
    targets: [
        .target(name: "UsageEngine"),
        .executableTarget(
            name: "Burnt",
            dependencies: ["UsageEngine"]
        ),
        .testTarget(
            name: "UsageEngineTests",
            dependencies: ["UsageEngine"],
            resources: [.copy("Fixtures")]
        ),
    ]
)
