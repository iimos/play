# libpg_query has no CMakeLists.txt so build it ourselves

CPMAddPackage("gh:pganalyze/libpg_query#17-6.0.0")

add_custom_target(build_libpg_query ALL
  COMMAND make build
  WORKING_DIRECTORY ${libpg_query_SOURCE_DIR}
)

add_library(libpg_query STATIC IMPORTED)
set(LIBPG_QUERY_INCLUDES
    "${libpg_query_SOURCE_DIR}"
    "${libpg_query_SOURCE_DIR}/src/include"
    "${libpg_query_SOURCE_DIR}/src/postgres/include"
    "${libpg_query_SOURCE_DIR}/protobuf"
)
set_target_properties(libpg_query PROPERTIES
  IMPORTED_LOCATION "${libpg_query_SOURCE_DIR}/libpg_query.a"
  INTERFACE_INCLUDE_DIRECTORIES "${LIBPG_QUERY_INCLUDES}"
)
add_dependencies(libpg_query build_libpg_query) # So that anyone linking against libpg_query causes build_libpg_query to build first

# file(GLOB LIBPG_QUERY_SOURCES
#         "${libpg_query_SOURCE_DIR}/src/*.c"
#         "${libpg_query_SOURCE_DIR}/src/postgres/*.c"
#         "${libpg_query_SOURCE_DIR}/protobuf/*.c"
#         "${libpg_query_SOURCE_DIR}/protobuf/*.cc"
# )
# add_library(libpg_query STATIC ${LIBPG_QUERY_SOURCES})
# target_include_directories(
#         libpg_query
#         PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}>
#         PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/src/include>
#         PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/src/postgres/include>
#         PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/vendor>
#         PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/protobuf>
# )
