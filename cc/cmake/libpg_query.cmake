# libpg_query has no CMakeLists.txt so build it ourselves

CPMAddPackage("gh:pganalyze/libpg_query#17-6.0.0")

file(GLOB LIBPG_QUERY_SOURCES
        "${libpg_query_SOURCE_DIR}/src/*.c"
        "${libpg_query_SOURCE_DIR}/src/postgres/*.c"
        "${libpg_query_SOURCE_DIR}/protobuf/*.c"
        "${libpg_query_SOURCE_DIR}/protobuf/*.cc"
)
add_library(libpg_query STATIC ${LIBPG_QUERY_SOURCES})
target_include_directories(
        libpg_query
        PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}>
        PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/src/include>
        PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/src/postgres/include>
        PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/vendor>
        PUBLIC $<BUILD_INTERFACE:${libpg_query_SOURCE_DIR}/protobuf>
)

find_package(Protobuf REQUIRED)
include_directories(${Protobuf_INCLUDE_DIRS})

include(cmake/FindProtobuf-c.cmake)
if (NOT PROTOBUF_C_FOUND)
    message(FATAL_ERROR "protobuf-c library not found")
endif()
