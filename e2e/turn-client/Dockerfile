ARG CI_GO_VERSION
FROM golang:${CI_GO_VERSION}

ADD . /src/turnc
WORKDIR /src/turnc/e2e/turn-client
RUN go install .

CMD ["turn-client"]
