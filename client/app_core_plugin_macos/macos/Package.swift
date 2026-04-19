// swift-tools-version: 5.9

import PackageDescription

let package = Package(
  name: "TunnelControlHarness",
  platforms: [
    .macOS(.v10_15),
  ],
  products: [
    .library(
      name: "TunnelControlHarness",
      targets: ["TunnelControlHarness"]
    ),
  ],
  targets: [
    .target(
      name: "TunnelControlHarness",
      path: "Classes/TunnelControl"
    ),
    .testTarget(
      name: "TunnelControlHarnessTests",
      dependencies: ["TunnelControlHarness"],
      path: "Tests/TunnelControlHarnessTests"
    ),
  ]
)
