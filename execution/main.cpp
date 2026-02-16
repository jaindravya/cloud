#include <iostream>
#include <string>
#include <cstring>


static std::string getArg(int argc, char* argv[], const char* key) {
    std::string k(key);
    for (int i = 1; i < argc - 1; ++i) {
        if (argv[i] == k && i + 1 < argc)
            return argv[i + 1];
    }
    return "";
}


int main(int argc, char* argv[]) {
    std::string jobId   = getArg(argc, argv, "--job-id");
    std::string type    = getArg(argc, argv, "--type");
    std::string payload = getArg(argc, argv, "--payload");


    if (payload.empty()) {
        std::cerr << "missing --payload\n";
        return 1;
    }


    // minimal C++ execution module for CPU-bound jobs
    // echo payload and succeed (demo), enables predictable execution times and clean separation from scheduler
    std::cout << "OK:" << payload << std::endl;
    return 0;
}





