# barcode

barcode is a simple microservice written in go: It features:

 - generate code128 barcode png

## URL Structure ##

imageproxy URLs are of the form `http://localhost:4000/code128/:width/:height/:content`.

### Docker ###

A docker image is available at [`alexes/barcode`](https://registry.hub.docker.com/u/alexes/barcode/dockerfile/).

You can run it by
```
docker run -p 4000:4000 alexes/barcode
```
