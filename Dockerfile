FROM --platform=linux/arm64 ubuntu:latest

# ARG GO_VERSION
ENV GO_VERSION=1.19

RUN apt-get update
RUN apt-get install -y wget git gcc strace gdb

RUN wget -P /tmp "https://dl.google.com/go/go${GO_VERSION}.linux-arm64.tar.gz"

RUN tar -C /usr/local -xzf "/tmp/go${GO_VERSION}.linux-arm64.tar.gz"
RUN rm "/tmp/go${GO_VERSION}.linux-arm64.tar.gz"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

WORKDIR $GOPATH

# dlv
# RUN go install github.com/go-delve/delve/cmd/dlv@latest

# clickhouse local
RUN apt-get install -y apt-transport-https ca-certificates dirmngr sudo
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 8919F6BD2B48D754
RUN echo "deb https://packages.clickhouse.com/deb stable main" | sudo tee /etc/apt/sources.list.d/clickhouse.list
#RUN apt-get install -y clickhouse-client
