FROM golang:latest

RUN go get gortc.io/gortcd
COPY gortcd.yml .

CMD ["gortcd"]
