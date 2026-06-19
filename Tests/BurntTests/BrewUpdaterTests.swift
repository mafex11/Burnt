// Tests/BurntTests/BrewUpdaterTests.swift
import XCTest
@testable import BurntCore

final class BrewUpdaterTests: XCTestCase {
    func testBrewManagedTrueWhenReceiptExists() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("brew-\(UUID().uuidString)")
        try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
        let receipt = tmp.appendingPathComponent("INSTALL_RECEIPT.json")
        try "{}".write(to: receipt, atomically: true, encoding: .utf8)
        let u = BrewUpdater(caskroomReceipt: receipt)
        XCTAssertTrue(u.isBrewManaged())
        try? fm.removeItem(at: tmp)
    }
    func testBrewManagedFalseWhenReceiptMissing() {
        let missing = FileManager.default.temporaryDirectory
            .appendingPathComponent("nope-\(UUID().uuidString).json")
        XCTAssertFalse(BrewUpdater(caskroomReceipt: missing).isBrewManaged())
    }
    func testBrewManagedFalseWhenReceiptNil() {
        XCTAssertFalse(BrewUpdater(caskroomReceipt: nil).isBrewManaged())
    }
    func testBrewPathResolvesFirstExistingCandidate() throws {
        let fm = FileManager.default
        let tmp = fm.temporaryDirectory.appendingPathComponent("brewbin-\(UUID().uuidString)")
        try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
        let fake = tmp.appendingPathComponent("brew")
        try "#!/bin/sh\n".write(to: fake, atomically: true, encoding: .utf8)
        try fm.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fake.path)
        let u = BrewUpdater(caskroomReceipt: nil,
                            brewCandidates: ["/no/such/brew", fake.path])
        XCTAssertEqual(u.brewPath(), fake.path)
        try? fm.removeItem(at: tmp)
    }
    func testBrewPathNilWhenNoCandidateExists() {
        let u = BrewUpdater(caskroomReceipt: nil, brewCandidates: ["/no/such/brew"])
        XCTAssertNil(u.brewPath())
    }
}
