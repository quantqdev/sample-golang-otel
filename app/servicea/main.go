package main

import (
	"context"
	"fmt"
	"log"
	"myorg/lib/otel"
	pb "myorg/lib/proto/gen/go/echo"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/logging/logrus"
	"gorm.io/plugin/opentelemetry/tracing"
)

const (
	serviceAddr      = ":8080"
	serviceName      = "servicea"
	serviceBGrpcAddr = "localhost:9091"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracer, meter, otelShutdown := otel.InitTracerMeter(ctx, serviceName)
	defer otelShutdown()

	testSpan(ctx, tracer, meter)
	startServer()
}

type EchoBody struct {
	Text string `json:"text" binding:"required"`
}

func startServer() {
	r := gin.New()
	r.Use(otelgin.Middleware(serviceName))
	r.POST("echo", func(ctx *gin.Context) {
		var body EchoBody
		if err := ctx.BindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"Message": "invalid body",
			})
			return
		}
		res, err := sendEcho(ctx.Request.Context(), body.Text)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"Error": err.Error(),
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"Message": res,
		})
	})
	_ = r.Run(serviceAddr)
}

func testSpan(ctx context.Context, tracer trace.Tracer, meter metric.Meter) {
	// Attributes represent additional key-value descriptors that can be bound
	// to a metric observer or recorder.
	commonAttrs := []attribute.KeyValue{
		attribute.String("attrA", "chocolate"),
		attribute.String("attrB", "raspberry"),
		attribute.String("attrC", "vanilla"),
	}

	runCount, err := meter.Int64Counter("run", metric.WithDescription("The number of times the iteration ran"))
	if err != nil {
		log.Fatal(err)
	}

	// Work begins
	ctx, span := tracer.Start(
		ctx,
		"testspan",
		trace.WithAttributes(commonAttrs...))
	n := 2
	for i := 0; i < n; i++ {
		_, iSpan := tracer.Start(ctx, fmt.Sprintf("Sample-%d", i))
		runCount.Add(ctx, 1, metric.WithAttributes(commonAttrs...))
		log.Printf("Doing really hard work (%d / %v)\n", i+1, n)

		<-time.After(time.Second)
		iSpan.End()
	}
	res, err := sendEcho(ctx, "testspan")
	if err != nil {
		otel.LogWithTraceID(ctx, err.Error())
	} else {
		otel.LogWithTraceID(ctx, *res)
	}
	queryDB(ctx, tracer)
	span.End()
}

func sendEcho(ctx context.Context, text string) (*string, error) {
	conn, err := grpc.NewClient(serviceBGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	c := pb.NewEchoServiceClient(conn)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	r, err := c.Echo(ctx, &pb.StringMessage{
		Value: text,
	})
	if err != nil {
		return nil, err
	}

	return &r.Value, nil
}

func queryDB(ctx context.Context, tracer trace.Tracer) {
	logger := logger.New(
		logrus.NewWriter(),
		logger.Config{
			SlowThreshold: time.Millisecond,
			LogLevel:      logger.Warn,
			Colorful:      false,
		},
	)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger})
	if err != nil {
		panic(err)
	}

	if err := db.Use(tracing.NewPlugin()); err != nil {
		panic(err)
	}

	ctx, span := tracer.Start(ctx, "root")
	defer span.End()

	var num int
	if err := db.WithContext(ctx).Raw("SELECT 42").Scan(&num).Error; err != nil {
		panic(err)
	}
}
