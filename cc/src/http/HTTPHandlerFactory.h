#pragma once

#include "Poco/Net/HTTPRequestHandlerFactory.h"

namespace http
{
    class HTTPHandlerFactory: public Poco::Net::HTTPRequestHandlerFactory
    {
    public:
        Poco::Net::HTTPRequestHandler* createRequestHandler(const Poco::Net::HTTPServerRequest& request) override;
    };
}