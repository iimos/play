#include "Factory.h"
#include "Ping.h"

#include <Poco/Net/HTTPServerRequest.h>
#include <Poco/Net/HTTPServerResponse.h>

namespace handlers
{
    Poco::Net::HTTPRequestHandler* Factory::createRequestHandler(const Poco::Net::HTTPServerRequest & req)
    {
        std::cout << "http: " << req.getMethod() << " " << req.getURI() << std::endl;

        auto const not_implemented = "501 Not Implemented\n";

        if (req.getMethod() != Poco::Net::HTTPRequest::HTTP_GET) {
            req.response().setStatus(Poco::Net::HTTPServerResponse::HTTP_NOT_IMPLEMENTED);
            req.response().send().write(not_implemented, static_cast<std::streamsize>(strlen(not_implemented)));
            req.response().send().flush();
            return nullptr;
        }

        if (req.getURI() == "/ping") {
            return new Ping();
        }

        req.response().setStatus(Poco::Net::HTTPServerResponse::HTTP_NOT_IMPLEMENTED);
        req.response().send().write(not_implemented, static_cast<std::streamsize>(strlen(not_implemented)));
        req.response().send().flush();
        return nullptr;
    }
}