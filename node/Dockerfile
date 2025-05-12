FROM golang:1.23

RUN apt-get update && apt-get install -y git
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod tidy

COPY . .

RUN go mod tidy
RUN go build -o node .
RUN chmod +x ./node

CMD ["/app/node"]