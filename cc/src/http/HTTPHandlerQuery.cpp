#include "Poco/Net/HTTPServerRequest.h"
#include "Poco/Net/HTTPServerResponse.h"
#include "HTTPHandlerQuery.h"
#include "pg_query.h"
#include "fmt/core.h"

namespace http
{
    void HTTPHandlerQuery::handleRequest(Poco::Net::HTTPServerRequest & req, Poco::Net::HTTPServerResponse & resp)
    {
        std::istream& in = req.stream();
        auto size = req.getContentLength();
        char buffer[size + 1];
        in.read(buffer, size);
        buffer[size] = '\0';
        if (in) {
            std::cout << "HTTP body: size=" << size << ", data: " << buffer << std::endl;
        }

        auto result = pg_query_parse(buffer);
        if (result.error) {
            fmt::println("error: {} at {}", result.error->message, result.error->cursorpos);
            resp.setContentType("text/txt");
            resp.setStatus(Poco::Net::HTTPResponse::HTTP_BAD_REQUEST);
            resp.send() << buffer << std::endl;
            return;
        }

        fmt::println("parse_tree: {}", result.parse_tree);
        pg_query_free_parse_result(result);

        resp.setContentType("text/txt");
        resp.setStatus(Poco::Net::HTTPResponse::HTTP_OK);
        resp.send() << buffer << std::endl;
    }
}