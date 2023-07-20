#
# To learn more about a Podspec see http://guides.cocoapods.org/syntax/podspec.html.
# Run `pod lib lint golib_plugin.podspec` to validate before publishing.
#
Pod::Spec.new do |s|
  s.name             = 'golib_plugin'
  s.version          = '0.0.1'
  s.summary          = 'Bison Relay golib plugin.'
  s.description      = <<-DESC
A flutter plugin that links to the ios Bison Relay golib.
                       DESC
  s.homepage         = 'https://bisonrelay.org'
  s.license          = { :file => '../LICENSE' }
  s.author           = { 'Company Zero' => 'info@companyzero.com' }
  s.source           = { :path => '.' }
  s.source_files = 'Classes/**/*'
  s.dependency 'Flutter'
  s.platform = :ios, '8.0'
  s.libraries = 'resolv.9'

  s.ios.vendored_frameworks = 'Frameworks/Golib.xcframework'

  # Flutter.framework does not contain a i386 slice.
  s.pod_target_xcconfig = { 'DEFINES_MODULE' => 'YES', 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'i386' }
  s.swift_version = '5.0'
end
