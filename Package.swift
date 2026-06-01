// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Burnt",
    platforms: [.macOS(.v14)],
    targets: [
        .target(name: "UsageEngine"),
        .target(name: "BurntCore", dependencies: ["UsageEngine"]),
        .executableTarget(
            name: "Burnt",
            dependencies: ["UsageEngine", "BurntCore"]
        ),
        .testTarget(
            name: "UsageEngineTests",
            dependencies: ["UsageEngine"],
            resources: [.copy("Fixtures")]
        ),
        .testTarget(
            name: "BurntTests",
            dependencies: ["BurntCore"]
        ),
    ]
)
