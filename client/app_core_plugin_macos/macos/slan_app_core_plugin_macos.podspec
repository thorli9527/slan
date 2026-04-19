Pod::Spec.new do |s|
  s.name             = 'slan_app_core_plugin_macos'
  s.version          = '0.1.0'
  s.summary          = 'macOS bridge from Flutter MethodChannel to the Rust app_core helper.'
  s.description      = <<-DESC
Forwards Flutter app_core facade calls to a long-lived Rust helper process over JSON stdio.
                       DESC
  s.homepage         = 'https://example.invalid/slan'
  s.license          = { :type => 'MIT', :file => '../../app_core_plugin/LICENSE' }
  s.author           = { 'SLAN' => 'devnull@example.invalid' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'FlutterMacOS'
  s.platform = :osx, '10.11'
  s.pod_target_xcconfig = { 'DEFINES_MODULE' => 'YES' }
  s.swift_version = '5.0'
end
