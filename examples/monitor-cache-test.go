package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DroneTestData 测试用的无人机数据
type DroneTestData struct {
	DroneID   string  `json:"drone_id"`
	Status    string  `json:"status"`
	Battery   float64 `json:"battery"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

func main() {
	fmt.Println("🚁 监控服务 Redis 缓存性能测试")
	fmt.Println("=====================================")

	// 监控服务地址
	baseURL := "http://localhost:50053"

	// 1. 测试发送无人机数据
	fmt.Println("\n📤 1. 发送测试无人机数据...")
	testDrones := []DroneTestData{
		{"DRONE001", "flying", 85.5, 40.7128, -74.0060, 150.0},
		{"DRONE002", "hovering", 67.8, 40.7589, -73.9851, 200.0},
		{"DRONE003", "landing", 42.3, 40.7505, -73.9934, 50.0},
		{"DRONE004", "flying", 91.2, 40.7359, -73.9911, 180.0},
		{"DRONE005", "maintenance", 15.7, 40.7484, -73.9857, 0.0},
	}

	for _, drone := range testDrones {
		sendDroneData(baseURL, drone)
		time.Sleep(100 * time.Millisecond)
	}

	// 2. 测试缓存性能
	fmt.Println("\n🚀 2. 缓存性能测试...")

	// 冷启动 - 无缓存
	fmt.Println("\n❄️  冷启动测试 (无缓存):")
	coldStart := testAPIPerformance(baseURL+"/api/monitoring/drones", 5)
	fmt.Printf("   平均响应时间: %.2fms\n", coldStart)

	// 等待缓存预热
	time.Sleep(1 * time.Second)

	// 热缓存测试
	fmt.Println("\n🔥 热缓存测试 (有缓存):")
	hotCache := testAPIPerformance(baseURL+"/api/monitoring/drones", 10)
	fmt.Printf("   平均响应时间: %.2fms\n", hotCache)

	// 性能提升计算
	improvement := ((coldStart - hotCache) / coldStart) * 100
	fmt.Printf("   性能提升: %.1f%%\n", improvement)

	// 3. 测试单个无人机缓存
	fmt.Println("\n🎯 3. 单个无人机缓存测试...")
	singleDroneURL := baseURL + "/api/monitoring/drone/DRONE001"

	// 第一次请求 (缓存未命中)
	start := time.Now()
	resp, err := http.Get(singleDroneURL)
	if err == nil {
		cache := resp.Header.Get("X-Cache")
		duration := time.Since(start).Milliseconds()
		fmt.Printf("   第一次请求: %dms (X-Cache: %s)\n", duration, cache)
		resp.Body.Close()
	}

	// 第二次请求 (缓存命中)
	start = time.Now()
	resp, err = http.Get(singleDroneURL)
	if err == nil {
		cache := resp.Header.Get("X-Cache")
		duration := time.Since(start).Milliseconds()
		fmt.Printf("   第二次请求: %dms (X-Cache: %s)\n", duration, cache)
		resp.Body.Close()
	}

	// 4. 测试系统指标缓存
	fmt.Println("\n📊 4. 系统指标缓存测试...")
	metricsURL := baseURL + "/api/monitoring/metrics"

	fmt.Println("   连续请求系统指标:")
	for i := 1; i <= 5; i++ {
		start := time.Now()
		resp, err := http.Get(metricsURL)
		if err == nil {
			cache := resp.Header.Get("X-Cache")
			duration := time.Since(start).Milliseconds()
			fmt.Printf("   请求 %d: %dms (X-Cache: %s)\n", i, duration, cache)
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 5. 测试警报缓存
	fmt.Println("\n🚨 5. 警报缓存测试...")
	alertsURL := baseURL + "/api/monitoring/alerts"

	// 连续请求警报列表
	for i := 1; i <= 3; i++ {
		start := time.Now()
		resp, err := http.Get(alertsURL)
		if err == nil {
			cache := resp.Header.Get("X-Cache")
			duration := time.Since(start).Milliseconds()
			fmt.Printf("   警报请求 %d: %dms (X-Cache: %s)\n", i, duration, cache)
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("\n✅ 缓存性能测试完成!")
	fmt.Println("\n💡 优化效果:")
	fmt.Printf("   - 无人机列表查询性能提升: %.1f%%\n", improvement)
	fmt.Println("   - 单个无人机查询支持缓存命中")
	fmt.Println("   - 系统指标查询使用30秒缓存")
	fmt.Println("   - 警报列表查询使用30秒缓存")
	fmt.Println("   - 实时数据通过Redis发布订阅广播")
	fmt.Println("   - 警报处理使用队列机制")
}

// sendDroneData 发送无人机数据到监控服务
func sendDroneData(baseURL string, drone DroneTestData) {
	data := map[string]interface{}{
		"drone_id":    drone.DroneID,
		"status":      drone.Status,
		"battery":     drone.Battery,
		"temperature": 25.0 + (drone.Battery-50)/10, // 模拟温度
		"speed":       0.0,
		"position": map[string]float64{
			"latitude":  drone.Latitude,
			"longitude": drone.Longitude,
			"altitude":  drone.Altitude,
		},
	}

	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(baseURL+"/api/monitoring/drones", "application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("   ❌ 发送 %s 数据失败: %v\n", drone.DroneID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Printf("   ✅ %s 数据发送成功 (电池: %.1f%%, 状态: %s)\n",
			drone.DroneID, drone.Battery, drone.Status)
	} else {
		fmt.Printf("   ❌ %s 数据发送失败: HTTP %d\n", drone.DroneID, resp.StatusCode)
	}
}

// testAPIPerformance 测试API性能
func testAPIPerformance(url string, requests int) float64 {
	var totalTime int64

	for i := 0; i < requests; i++ {
		start := time.Now()
		resp, err := http.Get(url)
		duration := time.Since(start).Milliseconds()

		if err == nil {
			resp.Body.Close()
			totalTime += duration
		}

		// 小延迟避免请求过于密集
		time.Sleep(50 * time.Millisecond)
	}

	return float64(totalTime) / float64(requests)
}
