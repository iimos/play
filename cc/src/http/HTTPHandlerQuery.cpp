#include "Poco/Net/HTTPServerRequest.h"
#include "Poco/Net/HTTPServerResponse.h"
#include "spdlog/spdlog.h"
#include "pg_query.h"
#include "pg_query.pb-c.h"
#include "protobuf-c/protobuf-c.h"
#include "HTTPHandlerQuery.h"

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
            SPDLOG_INFO("HTTP body: size={}, data: {}", size, static_cast<char*>(buffer));
        }

        auto result = pg_query_parse(buffer);
        if (result.error) {
            SPDLOG_ERROR("error: {} at {}", result.error->message, result.error->cursorpos);
            resp.setContentType("text/txt");
            resp.setStatus(Poco::Net::HTTPResponse::HTTP_BAD_REQUEST);
            resp.send() << buffer << std::endl;
            pg_query_free_parse_result(result);
            return;
        }
        SPDLOG_INFO("parse_tree: {}", result.parse_tree);
        pg_query_free_parse_result(result);

        auto scan = pg_query_scan(buffer);
        if (scan.error) {
            SPDLOG_ERROR("error: {} at {}", scan.error->message, scan.error->cursorpos);
            resp.setContentType("text/txt");
            resp.setStatus(Poco::Net::HTTPResponse::HTTP_BAD_REQUEST);
            resp.send() << buffer << std::endl;
            pg_query_free_scan_result(scan);
            return;
        }

        PgQuery__ScanToken * scan_token;
        const ProtobufCEnumValue * token_kind;
        const ProtobufCEnumValue * keyword_kind;

        auto scan_result = pg_query__scan_result__unpack(nullptr, scan.pbuf.len, reinterpret_cast<const uint8_t *>(scan.pbuf.data));
        printf("  version: %d, tokens: %zu, size: %zu\n", scan_result->version, scan_result->n_tokens, scan.pbuf.len);
        for (int j = 0; j < scan_result->n_tokens; j++) {
            scan_token = scan_result->tokens[j];
            token_kind = protobuf_c_enum_descriptor_get_value(&pg_query__token__descriptor, scan_token->token);
            keyword_kind = protobuf_c_enum_descriptor_get_value(&pg_query__keyword_kind__descriptor, scan_token->keyword_kind);
            SPDLOG_INFO("  \"{1:{0}s} = [ {2:d}, {3:d}, {4:s}, {5:s} ]\"", scan_token->end - scan_token->start, &(buffer[scan_token->start]), scan_token->start, scan_token->end, token_kind->name, keyword_kind->name);
        }
        pg_query__scan_result__free_unpacked(scan_result, nullptr);

        resp.setContentType("text/txt");
        resp.setStatus(Poco::Net::HTTPResponse::HTTP_OK);
        resp.send() << buffer << std::endl;
    }
}