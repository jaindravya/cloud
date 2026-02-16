cmake_minimum_required(VERSION 3.14)
project(cloud_runner CXX)


set(CMAKE_CXX_STANDARD 17)


add_executable(runner main.cpp)
install(TARGETS runner DESTINATION bin)





