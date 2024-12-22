cmake_minimum_required(VERSION 3.10)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON) # For clang-tidy.
set(BUILD_SHARED_LIBS OFF) # We expect external libraries to be linked statically.
set(CMAKE_CXX_STANDARD 17) # Compile as C++17.
set(CMAKE_CXX_STANDARD_REQUIRED ON) # Require C++17 support.
set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} "${CMAKE_SOURCE_DIR}/third_party/")

project(Play
    VERSION 0.1
    LANGUAGES C CXX
)

add_subdirectory(third_party)
include_directories(third_party)

find_package(Poco REQUIRED COMPONENTS Foundation Util Net XML JSON)
message("Poco_LIBRARIES = ${Poco_LIBRARIES}")

add_executable(server
        src/server.cpp
        src/http/Server.cpp
        src/http/handlers/Factory.cpp
        src/http/handlers/Factory.h
        src/http/handlers/Ping.cpp
        src/http/handlers/Ping.h
)
target_link_libraries(server argparse ${Poco_LIBRARIES})

add_executable(shell
        src/shell.cpp
)
target_link_libraries(shell argparse utf8proc linenoise ${Poco_LIBRARIES})
