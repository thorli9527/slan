Pod::Spec.new do |s|
  s.name             = 'client_core_plugin'
  s.version          = '0.1.0'
  s.summary          = 'SLAN client core macOS bridge.'
  s.description      = 'macOS platform bridge for SLAN Client V2.'
  s.homepage         = 'https://example.invalid/slan'
  s.license          = { :type => 'MIT' }
  s.author           = { 'SLAN' => 'dev@slan.local' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'FlutterMacOS'
  s.platform = :osx, '10.14'
  s.swift_version = '5.0'
end
