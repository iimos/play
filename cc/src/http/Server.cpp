#include <iostream>
#include <Poco/AutoPtr.h>
#include <Poco/ConsoleChannel.h>
#include <Poco/PatternFormatter.h>
#include <Poco/FormattingChannel.h>
#include <Poco/Net/HTTPServer.h>
#include <Poco/Net/ServerSocket.h>
#include "Server.h"
#include "HTTPHandlerFactory.h"

namespace http {

    int Server::main(const std::vector<std::string> &args) {
        // Set up logging
        Poco::AutoPtr<Poco::ConsoleChannel> consoleChannel(new Poco::ConsoleChannel);
        Poco::AutoPtr<Poco::PatternFormatter> patternFormatter(new Poco::PatternFormatter);
        patternFormatter->setProperty("pattern", "[%Y-%m-%d %H:%M:%S] [%p] %t");
        Poco::AutoPtr<Poco::FormattingChannel> formattingChannel(
                new Poco::FormattingChannel(patternFormatter, consoleChannel));
        Poco::Logger &logger = Poco::Logger::get("ServerLogger");
        logger.setChannel(formattingChannel);

        logger.information("http server: initializing");

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

        logger.information("http server: listen %s", socket_addr.toString());
        server->start();
        waitForTerminationRequest();

        logger.information("http server: stopping");
        server->stopAll();
        return 0;
    }
}