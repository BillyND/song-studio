// swift-tools-version:5.9
// The native shell of Quê Hương Studio: a window around the Go server's web UI, plus Sparkle updates.
import PackageDescription

let package = Package(
    name: "QueHuongStudio",
    platforms: [.macOS(.v13)],
    dependencies: [
        .package(url: "https://github.com/sparkle-project/Sparkle", exact: "2.10.0"),
    ],
    targets: [
        .executableTarget(
            name: "QueHuongStudio",
            dependencies: [.product(name: "Sparkle", package: "Sparkle")],
            linkerSettings: [.unsafeFlags(["-Xlinker", "-rpath", "-Xlinker", "@executable_path/../Frameworks"])]
        ),
    ]
)
