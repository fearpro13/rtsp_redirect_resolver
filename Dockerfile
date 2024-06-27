FROM golang:1.22.4

WORKDIR /app

COPY *.go ./
COPY cmd cmd
COPY go.* ./

RUN go get && cd cmd && go build -o ../build/rtsp_redirect_resolver

ENTRYPOINT ["./build/rtsp_redirect_resolver"]