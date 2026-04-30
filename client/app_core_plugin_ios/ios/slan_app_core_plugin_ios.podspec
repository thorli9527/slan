Pod::Spec.new do |s|
  s.name             = 'slan_app_core_plugin_ios'
  s.version          = '0.1.0'
  s.summary          = 'iOS host integration for SLAN app_core.'
  s.description      = 'Federated iOS implementation scaffold for the SLAN app_core Flutter plugin.'
  s.homepage         = 'https://github.com/thorli9527/slan'
  s.license          = { :type => 'MIT' }
  s.author           = { 'SLAN' => 'dev@slan.local' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'Flutter'
  s.platform = :ios, '13.0'
  s.swift_version = '5.0'
end
