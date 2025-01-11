#include <iostream>
#include "Poco/Net/HTTPServerRequest.h"
#include "Poco/Net/HTTPServerResponse.h"

#include "HTTPHandlerFactory.h"
#include "HTTPHandlerQuery.h"
#include "HTTPHandlerPing.h"

namespace http
{
    Poco::Net::HTTPRequestHandler* HTTPHandlerFactory::createRequestHandler(const Poco::Net::HTTPServerRequest & req)
    {
        std::cout << "http: " << req.getMethod() << " " << req.getURI() << std::endl;

        // Query handler
        if (req.getMethod() == Poco::Net::HTTPRequest::HTTP_POST && req.getURI() == "/") {
            return new HTTPHandlerQuery();
        }

        // Ping handler
        if (req.getMethod() == Poco::Net::HTTPRequest::HTTP_GET && req.getURI() == "/ping") {
            return new HTTPHandlerPing();
        }

        static auto const not_implemented = "501 Not Implemented\n";
        req.response().setStatus(Poco::Net::HTTPServerResponse::HTTP_NOT_IMPLEMENTED);
        req.response().send().write(not_implemented, static_cast<std::streamsize>(strlen(not_implemented)));
        req.response().send().flush();
        return nullptr;
    }
}
