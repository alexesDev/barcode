FROM golang:1.10.3
WORKDIR /go/src/github.com/alexesDev/barcode/
RUN go get github.com/boombuler/barcode/... && go get github.com/boombuler/barcode/code128/...
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM scratch
COPY --from=0 /go/src/github.com/alexesDev/barcode/app .
CMD ["./app"]
