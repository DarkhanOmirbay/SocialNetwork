package main

import (
	"context"
	_go "darkhan/gen/go"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	go RunRest()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Server stopped")
}
func RunRest() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := _go.RegisterUserHandlerFromEndpoint(ctx, mux, "localhost:50051", opts)
	if err != nil {
		panic(err)
	}
	err = _go.RegisterPostHandlerFromEndpoint(ctx, mux, "localhost:44044", opts)
	if err != nil {
		panic(err)
	}
	log.Printf("server listening at 8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		panic(err)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Server stopped")
}
