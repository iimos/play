#include <iostream>
#include "http/Server.h"

//POCO_SERVER_MAIN(Server)

int main(int argc, char** argv) {
    http::Server app;
    return app.run(argc, argv);
}