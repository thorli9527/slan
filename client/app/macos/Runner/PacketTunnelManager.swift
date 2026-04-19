import Foundation
import NetworkExtension

final class PacketTunnelManager {
    static let shared = PacketTunnelManager()

    private init() {}

    func loadManagers(completion: @escaping (Result<[NETunnelProviderManager], Error>) -> Void) {
        NETunnelProviderManager.loadAllFromPreferences { managers, error in
            if let error {
                completion(.failure(error))
                return
            }
            completion(.success(managers ?? []))
        }
    }
}
