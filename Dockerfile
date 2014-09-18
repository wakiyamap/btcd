FROM roasbeef/btcd

# Install our statsd client.
RUN go get github.com/cactus/go-statsd-client/statsd

# Make a temporary directory for our manual build.
RUN mkdir -p /root/btcdmon/
ADD . /root/btcdmon/

# Install the btcdmon fork, and replace the default btcd binary.
WORKDIR /root/btcdmon/
RUN go build && mv btcdmon btcd && rm /gopath/bin/btcd && mv /root/btcdmon/btcd /gopath/bin/btcd

RUN rm -rf /root/btcdmon
