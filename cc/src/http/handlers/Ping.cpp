#include "Ping.h"

#include <Poco/Net/HTTPServerResponse.h>

namespace handlers
{

    void Ping::handleRequest(Poco::Net::HTTPServerRequest& req, Poco::Net::HTTPServerResponse& resp)
    {
        resp.setStatus(Poco::Net::HTTPServerResponse::HTTP_OK);
        auto const data = "ok\n";
        resp.send().write(data, static_cast<std::streamsize>(strlen(data)));
        resp.send().flush();
    }
}