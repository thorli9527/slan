import 'package:integration_test/integration_test.dart';

import 'device_activation_test.dart' as device_activation;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  device_activation.runDeviceActivationTest();
}
