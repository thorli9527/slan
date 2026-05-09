Pod::Spec.new do |s|
  s.name             = 'client_core_plugin'
  s.version          = '0.1.0'
  s.summary          = 'Platform bridge facade for SLAN client core v2.'
  s.description      = <<-DESC
Platform bridge facade for SLAN client core v2.
                       DESC
  s.homepage         = 'https://slan.local'
  s.license          = { :type => 'MIT' }
  s.author           = { 'SLAN' => 'dev@slan.local' }
  s.source           = { :path => '.' }
  s.source_files = 'Classes/**/*'
  ffi_xcframework = 'Frameworks/ClientCoreFfi.xcframework'
  xcconfig = {
    'DEFINES_MODULE' => 'YES',
    'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'x86_64'
  }
  if File.exist?(File.join(__dir__, ffi_xcframework))
    s.vendored_frameworks = ffi_xcframework
    xcconfig['OTHER_SWIFT_FLAGS'] = '$(inherited) -D SLAN_CLIENT_CORE_FFI'
  end
  s.dependency 'Flutter'
  s.frameworks = 'NetworkExtension'
  s.platform = :ios, '13.0'
  s.swift_version = '5.0'
  s.pod_target_xcconfig = xcconfig
end
