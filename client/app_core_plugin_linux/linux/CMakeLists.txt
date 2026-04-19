cmake_minimum_required(VERSION 3.13)

project(slan_app_core_plugin_linux LANGUAGES CXX)

set(PLUGIN_NAME "slan_app_core_plugin_linux_plugin")

add_library(${PLUGIN_NAME} SHARED
  "slan_app_core_plugin_linux_plugin.cc"
)

apply_standard_settings(${PLUGIN_NAME})
set_target_properties(${PLUGIN_NAME} PROPERTIES
  CXX_VISIBILITY_PRESET hidden)
target_compile_features(${PLUGIN_NAME} PUBLIC cxx_std_17)
target_include_directories(${PLUGIN_NAME} INTERFACE
  "${CMAKE_CURRENT_SOURCE_DIR}/include")
target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin)

set(slan_app_core_plugin_linux_bundled_libraries
  ""
  PARENT_SCOPE
)
