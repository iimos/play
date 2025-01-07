#pragma once

#include "Poco/Net/HTTPRequestHandler.h"

namespace http
{
    class HTTPHandlerPing : public Poco::Net::HTTPRequestHandler
    {
    public:
        void handleRequest(Poco::Net::HTTPServerRequest & request, Poco::Net::HTTPServerResponse & response) override;
    };
}
