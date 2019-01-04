package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	graphite "github.com/almariah/go-graphite-client"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/philipsahli/gontador.test/service"
)

var redisdb *redis.Client
var graphiteClient *graphite.Client
var traceID string
var isReady bool

const (
	port = ":3001"
)

type server struct{}

func (s *server) Count(ctx context.Context, in *pb.Empty) (*pb.CountReply, error) {
	log.Printf("Received grpc Count request")
	counter := incr()
	return &pb.CountReply{Counter: counter}, nil
}

func (s *server) GetCounter(ctx context.Context, in *pb.Empty) (*pb.CountReply, error) {
	log.Printf("Received grpc GetCounter request")
	counter := get()
	return &pb.CountReply{Counter: counter}, nil
}

func init() {

	// Configure Logger
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Try to connect a graphite server
	log.Printf("Init graphite.Client to %s:%s", os.Getenv("METRIC_HOST"), os.Getenv("METRIC_PORT"))
	port, _ := strconv.Atoi(os.Getenv("METRIC_PORT"))
	graphiteClient = &graphite.Client{
		Host:     os.Getenv("METRIC_HOST"),
		Port:     port,
		Protocol: "tcp",
	}
	err := graphiteClient.Connect()
	if err != nil {
		log.Fatal("Could not connect to graphite:", err)
	}
}

func main() {

	isReady = false
	traceID = getTraceID()

	redisdb = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_URL"),
	})

	_, err := redisdb.Ping().Result()
	if err == nil {
		isReady = true
	}

	log.Println("Process started with PID", os.Getpid())

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	log.Println("grpc started on port 3001")

	r := mux.NewRouter()
	r.HandleFunc("/counter", count).Methods("GET")
	r.HandleFunc("/health/ready", ready).Methods("GET")
	r.HandleFunc("/health/live", live).Methods("GET")

	srv := &http.Server{
		Addr:    "0.0.0.0:3000",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := server{}
	grpcServer := grpc.NewServer()
	// pb.CounterServer(grpcServer, &s)
	pb.RegisterCounterServer(grpcServer, &s)
	// Register reflection service on gRPC server.
	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("shutting down")
	os.Exit(0)
}
