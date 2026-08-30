#include <sstream>
#include <string_view>

#include "Poco/Net/HTTPServerRequest.h"
#include "Poco/Net/HTTPServerResponse.h"
#include "HTTPHandlerQuery.h"
#include "spdlog/spdlog.h"
#include "pg_query.h"
#include "pg_query.pb-c.h"
#include "protobuf-c/protobuf-c.h"

namespace http
{
    void HTTPHandlerQuery::handleRequest(Poco::Net::HTTPServerRequest & req, Poco::Net::HTTPServerResponse & resp)
    {
        auto size = req.getContentLength();
        std::string body;
        body.resize(size);
        req.stream().read(&body[0], size);

        SPDLOG_INFO("HTTP body: size={}, data: {}", size, body);

        // pg_query_raw_parse(buf.c_str(), 0);

        // MemoryContext context = pg_query_enter_memory_context();
        // PgQueryInternalParsetreeAndError result = pg_query_raw_parse(std::string{sql}.c_str());

        // auto result = pg_query_parse(body.c_str());
        // if (result.error) {
        //     SPDLOG_ERROR("error: {} at {}", result.error->message, result.error->cursorpos);
        //     resp.setContentType("text/txt");
        //     resp.setStatus(Poco::Net::HTTPResponse::HTTP_BAD_REQUEST);
        //     resp.send() << body << std::endl;
        //     pg_query_free_parse_result(result);
        //     return;
        // }
        //
        // SPDLOG_INFO("parse_tree: {}", result.parse_tree);
        // pg_query_free_parse_result(result);

        auto scan = pg_query_scan(body.c_str());
        if (scan.error) {
            SPDLOG_ERROR("error: {} at {}", scan.error->message, scan.error->cursorpos);
            resp.setContentType("text/txt");
            resp.setStatus(Poco::Net::HTTPResponse::HTTP_BAD_REQUEST);
            resp.send() << body << std::endl;
            pg_query_free_scan_result(scan);
            return;
        }

        PgQuery__ScanToken * scan_token;
        const ProtobufCEnumValue * token_kind;
        const ProtobufCEnumValue * keyword_kind;
        const std::string_view body_view {body};

        auto scan_result = pg_query__scan_result__unpack(nullptr, scan.pbuf.len, reinterpret_cast<const uint8_t *>(scan.pbuf.data));
        printf("  version: %d, tokens: %zu, size: %zu\n", scan_result->version, scan_result->n_tokens, scan.pbuf.len);
        for (int j = 0; j < scan_result->n_tokens; j++) {
            scan_token = scan_result->tokens[j];

            if (j == 0 && scan_token->token == PG_QUERY__TOKEN__SELECT) {
                SPDLOG_INFO("IT IS SELECT {}", static_cast<int>(scan_token->token));
            }

            token_kind = protobuf_c_enum_descriptor_get_value(&pg_query__token__descriptor, scan_token->token);
            keyword_kind = protobuf_c_enum_descriptor_get_value(&pg_query__keyword_kind__descriptor, scan_token->keyword_kind);
            auto substr = body_view.substr(scan_token->start, scan_token->end - scan_token->start);
            SPDLOG_INFO("\t{0:s}\t{1:s}\t{2:s}", substr, token_kind->name, keyword_kind->name);
        }
        pg_query__scan_result__free_unpacked(scan_result, nullptr);

        resp.setContentType("text/txt");
        resp.setStatus(Poco::Net::HTTPResponse::HTTP_OK);
        resp.send() << body << std::endl;
    }
}
