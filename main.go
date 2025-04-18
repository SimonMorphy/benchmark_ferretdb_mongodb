package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BenchmarkConfig struct {
	FerretDBURI string
	MongoDBURI  string
	Database    string
	Collection  string
	Concurrent  int
	DocSize     int
	Duration    time.Duration
	Operation   string
	ReadWrite   float64
	NoWarmup    bool
}

type BenchmarkResult struct {
	Operation     string
	Database      string
	Concurrent    int
	DocSize       int
	OpsPerSec     float64
	AvgLatency    float64
	P95Latency    float64
	P99Latency    float64
	Errors        int64
	TimeElapsed   time.Duration
	WarmupSkipped bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds) // 添加微秒级时间戳
	config := parseFlags()

	log.Printf("Starting benchmark with configuration:")
	log.Printf("- Operation: %s", config.Operation)
	log.Printf("- Duration: %v", config.Duration)
	log.Printf("- Concurrent connections: %d", config.Concurrent)
	log.Printf("- Document size: %d bytes", config.DocSize)
	log.Printf("- Warmup: %v", !config.NoWarmup)

	// 运行 FerretDB 测试
	log.Printf("\nStarting FerretDB benchmark...")
	ferretResult := runBenchmark("FerretDB", config)
	printResult(ferretResult)

	// 运行 MongoDB 测试
	log.Printf("\nStarting MongoDB benchmark...")
	mongoResult := runBenchmark("MongoDB", config)
	printResult(mongoResult)

	// 保存结果
	saveResults(ferretResult, mongoResult)
}

func parseFlags() *BenchmarkConfig {
	config := &BenchmarkConfig{}

	flag.StringVar(&config.FerretDBURI, "ferretdb", "mongodb://root:password@localhost:27017", "FerretDB connection URI")
	flag.StringVar(&config.MongoDBURI, "mongodb", "mongodb://root:password@localhost:27018", "MongoDB connection URI")
	flag.StringVar(&config.Database, "db", "benchmark", "Database name")
	flag.StringVar(&config.Collection, "collection", "test", "Collection name")
	flag.IntVar(&config.Concurrent, "concurrent", 10, "Number of concurrent connections")
	flag.IntVar(&config.DocSize, "docsize", 1024, "Document size in bytes")
	flag.DurationVar(&config.Duration, "duration", 5*time.Minute, "Test duration")
	flag.StringVar(&config.Operation, "op", "insert", "Operation type (insert/query/update/delete)")
	flag.Float64Var(&config.ReadWrite, "rw", 0.7, "Read/Write ratio (0.7 means 70% reads)")
	flag.BoolVar(&config.NoWarmup, "no-warmup", false, "Skip warmup phase")

	flag.Parse()
	return config
}

func runBenchmark(dbType string, config *BenchmarkConfig) *BenchmarkResult {
	uri := config.MongoDBURI
	if dbType == "FerretDB" {
		uri = config.FerretDBURI
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", dbType, err)
	}
	defer client.Disconnect(ctx)

	// 测试连接
	log.Printf("Testing connection to %s...", dbType)
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping %s: %v", dbType, err)
	}
	log.Printf("Successfully connected to %s", dbType)

	// 预热（根据配置决定是否跳过）
	if !config.NoWarmup {
		log.Printf("Warming up %s for 5 minutes...", dbType)
		time.Sleep(5 * time.Minute)
		log.Printf("Warmup completed for %s", dbType)
	} else {
		log.Printf("Skipping warmup for %s", dbType)
	}

	collection := client.Database(config.Database).Collection(config.Collection)

	var wg sync.WaitGroup
	results := make(chan time.Duration, config.Concurrent*1000)
	var errors int64
	var operations int64

	start := time.Now()
	deadline := start.Add(config.Duration)

	// 启动进度报告协程
	stopProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ops := atomic.LoadInt64(&operations)
				errs := atomic.LoadInt64(&errors)
				elapsed := time.Since(start)
				if elapsed > 0 {
					log.Printf("[Progress] %s - Ops: %d, Errors: %d, Rate: %.2f ops/sec",
						dbType, ops, errs, float64(ops)/elapsed.Seconds())
				}
			case <-stopProgress:
				return
			}
		}
	}()

	// 启动工作协程
	log.Printf("Starting %d worker goroutines for %s...", config.Concurrent, dbType)
	for i := 0; i < config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				opStart := time.Now()
				var err error

				switch config.Operation {
				case "insert":
					err = doInsert(ctx, collection, config.DocSize)
				case "query":
					err = doQuery(ctx, collection)
				case "update":
					err = doUpdate(ctx, collection)
				case "delete":
					err = doDelete(ctx, collection)
				}

				if err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}

				atomic.AddInt64(&operations, 1)
				results <- time.Since(opStart)
			}
		}()
	}

	wg.Wait()
	close(stopProgress)
	close(results)

	log.Printf("Benchmark completed for %s", dbType)

	// 计算统计数据
	var latencies []time.Duration
	for latency := range results {
		latencies = append(latencies, latency)
	}

	return &BenchmarkResult{
		Operation:     config.Operation,
		Database:      dbType,
		Concurrent:    config.Concurrent,
		DocSize:       config.DocSize,
		OpsPerSec:     float64(operations) / config.Duration.Seconds(),
		AvgLatency:    calculateAvgLatency(latencies),
		P95Latency:    calculatePercentileLatency(latencies, 0.95),
		P99Latency:    calculatePercentileLatency(latencies, 0.99),
		Errors:        errors,
		TimeElapsed:   config.Duration,
		WarmupSkipped: config.NoWarmup,
	}
}

func doInsert(ctx context.Context, coll *mongo.Collection, docSize int) error {
	doc := generateDocument(docSize)
	_, err := coll.InsertOne(ctx, doc)
	return err
}

func doQuery(ctx context.Context, coll *mongo.Collection) error {
	filter := bson.M{"_id": generateRandomID()}
	_, err := coll.FindOne(ctx, filter).DecodeBytes()
	return err
}

func doUpdate(ctx context.Context, coll *mongo.Collection) error {
	filter := bson.M{"_id": generateRandomID()}
	update := bson.M{"$set": bson.M{"updated_at": time.Now()}}
	_, err := coll.UpdateOne(ctx, filter, update)
	return err
}

func doDelete(ctx context.Context, coll *mongo.Collection) error {
	filter := bson.M{"_id": generateRandomID()}
	_, err := coll.DeleteOne(ctx, filter)
	return err
}

func generateDocument(size int) bson.M {
	doc := bson.M{
		"_id":        generateRandomID(),
		"created_at": time.Now(),
		"data":       generateRandomString(size - 100), // 预留一些空间给其他字段
	}
	return doc
}

func generateRandomID() string {
	return fmt.Sprintf("%x", rand.Int63())
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func calculateAvgLatency(latencies []time.Duration) float64 {
	if len(latencies) == 0 {
		return 0
	}
	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	return float64(sum) / float64(len(latencies)) / float64(time.Millisecond)
}

func calculatePercentileLatency(latencies []time.Duration, percentile float64) float64 {
	if len(latencies) == 0 {
		return 0
	}

	// 排序延迟数据
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	idx := int(float64(len(sorted)) * percentile)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return float64(sorted[idx]) / float64(time.Millisecond)
}

func printResult(result *BenchmarkResult) {
	fmt.Printf("\nResults for %s (%s):\n", result.Database, result.Operation)
	if result.WarmupSkipped {
		fmt.Printf("Warmup: Skipped\n")
	} else {
		fmt.Printf("Warmup: Completed (5 minutes)\n")
	}
	fmt.Printf("Operations/sec: %.2f\n", result.OpsPerSec)
	fmt.Printf("Average Latency: %.2f ms\n", result.AvgLatency)
	fmt.Printf("P95 Latency: %.2f ms\n", result.P95Latency)
	fmt.Printf("P99 Latency: %.2f ms\n", result.P99Latency)
	fmt.Printf("Errors: %d\n", result.Errors)
	fmt.Printf("Time Elapsed: %v\n", result.TimeElapsed)
}

func saveResults(ferretResult, mongoResult *BenchmarkResult) {
	results := struct {
		FerretDB *BenchmarkResult
		MongoDB  *BenchmarkResult
		Time     time.Time
	}{
		FerretDB: ferretResult,
		MongoDB:  mongoResult,
		Time:     time.Now(),
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	filename := fmt.Sprintf("results_%s_%d.json", ferretResult.Operation, time.Now().Unix())
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("Error saving results: %v", err)
	}
}
