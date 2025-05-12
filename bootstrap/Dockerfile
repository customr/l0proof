FROM golang:1.23

RUN apt-get update && apt-get install -y git
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod tidy

COPY . .

RUN go build -o bootstrap .
RUN chmod +x ./bootstrap

CMD ["/app/bootstrap"]