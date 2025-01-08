#include <Poco/AutoPtr.h>
#include <Poco/Net/HTTPServer.h>
#include <Poco/Net/ServerSocket.h>
#include "Server.h"
#include "HTTPHandlerFactory.h"
#include "spdlog/spdlog.h"

namespace http {

    int Server::main(const std::vector<std::string> &args) {
        // Set up logging
        spdlog::set_level(spdlog::level::debug);

        SPDLOG_INFO("http server: initializing");

        Poco::Net::HTTPServerParams::Ptr http_params = new Poco::Net::HTTPServerParams();
        http_params->setTimeout(std::chrono::seconds(30));
        http_params->setKeepAlive(true);
        http_params->setKeepAliveTimeout(std::chrono::seconds(10));
        http_params->setMaxKeepAliveRequests(10'000);
        http_params->setMaxQueued(100);
        http_params->setMaxThreads(4);

        static const std::string addr = "localhost:8080";
        const auto socket_addr = Poco::Net::SocketAddress(addr);
        auto socket = Poco::Net::ServerSocket(socket_addr);
        socket.setReuseAddress(true);
        socket.setReusePort(false);
        socket.setReceiveTimeout(std::chrono::seconds(30));
        socket.setSendTimeout(std::chrono::seconds(30));
        socket.listen(/* backlog = */ 64);

        auto server = new Poco::Net::HTTPServer(new http::HTTPHandlerFactory(), socket, http_params);

        SPDLOG_INFO("http server: listen {}", socket_addr.toString());
        server->start();
        waitForTerminationRequest();

        SPDLOG_INFO("http server: stopping");
        server->stopAll();
        return 0;
    }
}