// c++ job runner with built-in job types: hash (sha-256), prime, sleep.
// falls back to echo for unknown types so existing clients keep working.
// usage: runner --job-id id --type type --payload payload

#include <iostream>
#include <string>
#include <cstring>
#include <cmath>
#include <chrono>
#include <thread>
#include <sstream>
#include <iomanip>
#include <vector>
#include <cstdint>


static std::string getArg(int argc, char* argv[], const char* key) {
    std::string k(key);
    for (int i = 1; i < argc - 1; ++i) {
        if (argv[i] == k && i + 1 < argc)
            return argv[i + 1];
    }
    return "";
}

// ---- minimal sha-256 (no external deps) -----------------------------------

static const uint32_t sha256K[64] = {
    0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
    0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
    0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
    0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
    0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
    0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
    0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
    0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2
};

static uint32_t rotr(uint32_t x, int n) { return (x >> n) | (x << (32 - n)); }

static std::string sha256(const std::string& input) {
    uint32_t h0 = 0x6a09e667, h1 = 0xbb67ae85, h2 = 0x3c6ef372, h3 = 0xa54ff53a;
    uint32_t h4 = 0x510e527f, h5 = 0x9b05688c, h6 = 0x1f83d9ab, h7 = 0x5be0cd19;

    uint64_t bitLen = (uint64_t)input.size() * 8;
    std::vector<uint8_t> msg(input.begin(), input.end());
    msg.push_back(0x80);
    while ((msg.size() % 64) != 56)
        msg.push_back(0x00);
    for (int i = 7; i >= 0; --i)
        msg.push_back((uint8_t)(bitLen >> (i * 8)));

    for (size_t offset = 0; offset < msg.size(); offset += 64) {
        uint32_t w[64];
        for (int i = 0; i < 16; ++i)
            w[i] = ((uint32_t)msg[offset+i*4]<<24)|((uint32_t)msg[offset+i*4+1]<<16)|
                   ((uint32_t)msg[offset+i*4+2]<<8)|((uint32_t)msg[offset+i*4+3]);
        for (int i = 16; i < 64; ++i) {
            uint32_t s0 = rotr(w[i-15],7) ^ rotr(w[i-15],18) ^ (w[i-15]>>3);
            uint32_t s1 = rotr(w[i-2],17) ^ rotr(w[i-2],19) ^ (w[i-2]>>10);
            w[i] = w[i-16] + s0 + w[i-7] + s1;
        }
        uint32_t a=h0,b=h1,c=h2,d=h3,e=h4,f=h5,g=h6,h=h7;
        for (int i = 0; i < 64; ++i) {
            uint32_t S1 = rotr(e,6) ^ rotr(e,11) ^ rotr(e,25);
            uint32_t ch = (e & f) ^ (~e & g);
            uint32_t t1 = h + S1 + ch + sha256K[i] + w[i];
            uint32_t S0 = rotr(a,2) ^ rotr(a,13) ^ rotr(a,22);
            uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
            uint32_t t2 = S0 + maj;
            h=g; g=f; f=e; e=d+t1; d=c; c=b; b=a; a=t1+t2;
        }
        h0+=a; h1+=b; h2+=c; h3+=d; h4+=e; h5+=f; h6+=g; h7+=h;
    }

    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    oss << std::setw(8)<<h0 << std::setw(8)<<h1 << std::setw(8)<<h2 << std::setw(8)<<h3;
    oss << std::setw(8)<<h4 << std::setw(8)<<h5 << std::setw(8)<<h6 << std::setw(8)<<h7;
    return oss.str();
}

// ---- json helpers (minimal, no external deps) -----------------------------

// extract a string value for a given key from a flat json object
static std::string jsonStr(const std::string& json, const std::string& key) {
    std::string needle = "\"" + key + "\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return "";
    pos = json.find(':', pos + needle.size());
    if (pos == std::string::npos) return "";
    pos = json.find('"', pos + 1);
    if (pos == std::string::npos) return "";
    auto end = json.find('"', pos + 1);
    if (end == std::string::npos) return "";
    return json.substr(pos + 1, end - pos - 1);
}

// extract an integer/float value for a given key from a flat json object
static double jsonNum(const std::string& json, const std::string& key) {
    std::string needle = "\"" + key + "\"";
    auto pos = json.find(needle);
    if (pos == std::string::npos) return 0;
    pos = json.find(':', pos + needle.size());
    if (pos == std::string::npos) return 0;
    ++pos;
    while (pos < json.size() && json[pos] == ' ') ++pos;
    return std::stod(json.substr(pos));
}

// ---- job type: hash -------------------------------------------------------

static int runHash(const std::string& payload) {
    std::string input = jsonStr(payload, "input");
    if (input.empty()) {
        std::cerr << "hash payload requires non-empty \"input\" field\n";
        return 1;
    }
    std::cout << sha256(input) << std::endl;
    return 0;
}

// ---- job type: prime ------------------------------------------------------

static const int maxPrimeN = 100000000;

static int runPrime(const std::string& payload) {
    int n = (int)jsonNum(payload, "n");
    if (n < 2) {
        std::cout << "primes_up_to=0 count=0 elapsed=0ms" << std::endl;
        return 0;
    }
    if (n > maxPrimeN) {
        std::cerr << "n=" << n << " exceeds maximum allowed value of " << maxPrimeN << "\n";
        return 1;
    }

    auto start = std::chrono::steady_clock::now();

    // sieve of eratosthenes
    std::vector<bool> composite(n + 1, false);
    int limit = (int)std::sqrt((double)n);
    for (int i = 2; i <= limit; ++i) {
        if (!composite[i]) {
            for (int j = i * i; j <= n; j += i)
                composite[j] = true;
        }
    }
    int count = 0;
    for (int i = 2; i <= n; ++i) {
        if (!composite[i]) ++count;
    }

    auto elapsed = std::chrono::steady_clock::now() - start;
    long ms = std::chrono::duration_cast<std::chrono::milliseconds>(elapsed).count();

    std::cout << "primes_up_to=" << n << " count=" << count << " elapsed=" << ms << "ms" << std::endl;
    return 0;
}

// ---- job type: sleep ------------------------------------------------------

static const int maxSleepSeconds = 300;

static int runSleep(const std::string& payload) {
    double seconds = jsonNum(payload, "seconds");
    if (seconds <= 0) {
        std::cerr << "sleep seconds must be > 0\n";
        return 1;
    }
    if (seconds > maxSleepSeconds) {
        std::cerr << "sleep seconds " << seconds << " exceeds maximum of " << maxSleepSeconds << "\n";
        return 1;
    }

    int ms = (int)(seconds * 1000);
    std::this_thread::sleep_for(std::chrono::milliseconds(ms));
    std::cout << "slept for " << ms << "ms" << std::endl;
    return 0;
}

// ---- main -----------------------------------------------------------------

int main(int argc, char* argv[]) {
    std::string jobId   = getArg(argc, argv, "--job-id");
    std::string type    = getArg(argc, argv, "--type");
    std::string payload = getArg(argc, argv, "--payload");

    (void)jobId;

    if (payload.empty()) {
        std::cerr << "missing --payload\n";
        return 1;
    }

    if (type == "hash")  return runHash(payload);
    if (type == "prime") return runPrime(payload);
    if (type == "sleep") return runSleep(payload);

    // unknown type: fall back to echo (backwards compatible)
    std::cout << "OK:" << payload << std::endl;
    return 0;
}
