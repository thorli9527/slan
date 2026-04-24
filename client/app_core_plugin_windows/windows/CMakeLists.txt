cmake_minimum_required(VERSION 3.14)

project(slan_app_core_plugin_windows LANGUAGES CXX)

set(PLUGIN_NAME "slan_app_core_plugin_windows_plugin")

add_library(${PLUGIN_NAME} SHARED
  "slan_app_core_plugin_windows_plugin.cpp"
  "slan_app_core_plugin_windows_plugin_c_api.cpp"
)

apply_standard_settings(${PLUGIN_NAME})
set_target_properties(${PLUGIN_NAME} PROPERTIES
  CXX_VISIBILITY_PRESET hidden)
target_compile_features(${PLUGIN_NAME} PUBLIC cxx_std_17)
target_compile_definitions(${PLUGIN_NAME} PRIVATE
  SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_IMPL)
target_include_directories(${PLUGIN_NAME} INTERFACE
  "${CMAKE_CURRENT_SOURCE_DIR}/include")
target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin ws2_32)

set(slan_app_core_plugin_windows_bundled_libraries
  ""
  PARENT_SCOPE
)
