cmake_minimum_required(VERSION 3.14)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON) # For clang-tidy.
set(BUILD_SHARED_LIBS OFF) # We expect external libraries to be linked statically.
set(CMAKE_CXX_STANDARD 17) # Compile as C++17.
set(CMAKE_CXX_STANDARD_REQUIRED ON) # Require C++17 support.
set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} "${CMAKE_SOURCE_DIR}/third_party/")

project(Play
    VERSION 0.1
    LANGUAGES C CXX
)

include(cmake/CPM.cmake)

CPMAddPackage("gh:fmtlib/fmt#11.1.1")
CPMAddPackage("gh:gabime/spdlog@1.15.0")

CPMAddPackage("gh:antirez/linenoise#1.0")
add_library(linenoise STATIC ${linenoise_SOURCE_DIR}/linenoise.c)
target_include_directories(linenoise SYSTEM PUBLIC ${linenoise_SOURCE_DIR})

include(cmake/libpg_query.cmake)

CPMAddPackage(
        NAME Poco
        VERSION 1.14.0
        URL https://github.com/pocoproject/poco/archive/refs/tags/poco-1.14.0-release.tar.gz
        OPTIONS
        "BUILD_SHARED_LIBS OFF"
        "ENABLE_FOUNDATION ON"
        "ENABLE_XML OFF"
        "ENABLE_JSON ON"
        "ENABLE_NET ON"
        "ENABLE_NETSSL OFF"
        "ENABLE_CRYPTO OFF"
        "ENABLE_JWT OFF"
        "ENABLE_DATA OFF"
        "ENABLE_DATA_SQLITE OFF"
        "ENABLE_DATA_MYSQL OFF"
        "ENABLE_DATA_POSTGRESQL OFF"
        "ENABLE_DATA_ODBC OFF"
        "ENABLE_MONGODB OFF"
        "ENABLE_PROMETHEUS OFF"
        "ENABLE_REDIS OFF"
        "ENABLE_PDF OFF"
        "ENABLE_UTIL ON"
        "ENABLE_ZIP OFF"
        "ENABLE_SEVENZIP OFF"
        "ENABLE_ACTIVERECORD OFF"
        "ENABLE_ACTIVERECORD_COMPILER OFF"
        "ENABLE_APACHECONNECTOR OFF"
        "ENABLE_CPPPARSER OFF"
        "ENABLE_ENCODINGS OFF"
        "ENABLE_ENCODINGS_COMPILER OFF"
        "ENABLE_PAGECOMPILER OFF"
        "ENABLE_PAGECOMPILER_FILE2PAGE OFF"
        "ENABLE_POCODOC OFF"
        "ENABLE_TESTS OFF"
        "ENABLE_SAMPLES OFF"
        "ENABLE_LONG_RUNNING_TESTS OFF"
        "POCO_UNBUNDLED OFF"
)

CPMAddPackage(
        NAME Boost
        VERSION 1.87.0
        URL https://github.com/boostorg/boost/releases/download/boost-1.87.0/boost-1.87.0-cmake.tar.xz
        URL_HASH SHA256=7da75f171837577a52bbf217e17f8ea576c7c246e4594d617bfde7fafd408be5
        OPTIONS "BOOST_ENABLE_CMAKE ON" "BOOST_SKIP_INSTALL_RULES ON" # Set `OFF` for installation
        "BUILD_SHARED_LIBS OFF" "BOOST_INCLUDE_LIBRARIES container\\\;stacktrace" # Note the escapes!
)

# find_package(Poco REQUIRED COMPONENTS Foundation Util Net XML JSON)

add_executable(server
        src/server.cpp
        src/http/Server.cpp
        src/http/HTTPHandlerFactory.cpp
        src/http/HTTPHandlerFactory.h
        src/http/HTTPHandlerPing.cpp
        src/http/HTTPHandlerPing.h
        src/http/HTTPHandlerQuery.cpp
        src/http/HTTPHandlerQuery.h
)
target_link_libraries(
    server
        fmt
        spdlog
        Poco::Foundation
        Poco::Net
        Poco::Util
        Boost::stacktrace
        libpg_query
        ${PROTOBUF_C_LIBRARY}
)

add_executable(shell
        src/shell.cpp
)
target_link_libraries(
    shell
        linenoise
        Poco::Foundation
        Poco::Net
        Poco::Util
)
