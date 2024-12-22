#include <iostream>
#include <string>
#include "linenoise/linenoise.h"

inline bool ends_with(std::string const & value, std::string const & ending) {
    if (ending.size() > value.size()) {
		return false;
	}
    return std::equal(ending.rbegin(), ending.rend(), value.rbegin());
}

int main(int argc, char** argv) {
    linenoiseHistorySetMaxLen(1024);
	linenoiseSetMultiLine(1);

	auto prompt = "shell> ";

	while (true) {
		std::string query;
		bool first_line = true;
		while (true) {
			auto line_prompt = first_line ? prompt : "... ";
            auto line = linenoise(line_prompt);
            if (line == nullptr) {
                return 0;
            }
			query += line;
            linenoiseFree(line);
			if (ends_with(query, ";")) {
				break;
			}
			query += " ";
			first_line = false;
		}
        linenoiseHistoryAdd(query.c_str());
		std::cout << "\t\tquery: " << query << std::endl;
	}
}

