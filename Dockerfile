FROM golang

ENV PORT 8080
ENV MONGO_HOST mongo

EXPOSE $PORT

COPY Godeps /go/src/app/Godeps

RUN go get github.com/tools/godep
WORKDIR /go/src/app
RUN godep restore

COPY . /go/src/app
RUN go build .

CMD ["/go/src/app/app"]