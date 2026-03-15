package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// generateCloudflareKey 根据逆向分析的算法生成 API 请求所需的 key 和 time
func generateCloudflareKey() (key string, timestamp string) {
	// 1. 获取当前时间的毫秒级时间戳
	timeMillis := time.Now().UnixNano() / int64(time.Millisecond)
	timestamp = strconv.FormatInt(timeMillis, 10)

	// 2. 定义两个固定的密钥字符串
	secret1 := "DdlTxtN0sUOu"
	secret2 := "70cloudflareapikey"

	// 3. 计算 secret1 的 MD5 哈希值 (h1)
	hasher1 := md5.New()
	hasher1.Write([]byte(secret1))
	h1 := hex.EncodeToString(hasher1.Sum(nil))

	// 4. 拼接最终字符串 (s_final = h1 + secret2 + time)
	finalInput := h1 + secret2 + timestamp

	// 5. 计算最终字符串的 MD5 哈希值，作为 key
	hasher2 := md5.New()
	hasher2.Write([]byte(finalInput))
	key = hex.EncodeToString(hasher2.Sum(nil))

	return key, timestamp
}

// verifyWithKnownValues 使用之前截获的已知值来验证我们的算法是否正确
func verifyWithKnownValues() {
	fmt.Println("--- 正在使用已知值验证算法 ---")
	knownTime := "1759080608746"
	expectedKey := "30facf8080b147f5122f8a8b3a6f1cd7"

	secret1 := "DdlTxtN0sUOu"
	secret2 := "70cloudflareapikey"

	// 执行与 generateCloudflareKey 中完全相同的步骤
	hasher1 := md5.New()
	hasher1.Write([]byte(secret1))
	h1 := hex.EncodeToString(hasher1.Sum(nil))

	finalInput := h1 + secret2 + knownTime

	hasher2 := md5.New()
	hasher2.Write([]byte(finalInput))
	calculatedKey := hex.EncodeToString(hasher2.Sum(nil))

	fmt.Printf("已知 Time: %s\n", knownTime)
	fmt.Printf("期望 Key:  %s\n", expectedKey)
	fmt.Printf("计算 Key:  %s\n", calculatedKey)

	if calculatedKey == expectedKey {
		fmt.Println(">>> 验证成功！算法正确。\n")
	} else {
		fmt.Println(">>> 验证失败！算法不匹配。\n")
	}
}

func main() {
	// 首先，用已知数据验证我们的算法是否正确
	verifyWithKnownValues()

	// 然后，生成新的 key 和 time 并请求 API
	fmt.Println("--- 正在生成新参数并请求 API ---")
	key, timestamp := generateCloudflareKey()

	// 构建 API URL
	apiURL := fmt.Sprintf("https://api.uouin.com/index.php/index/Cloudflare?key=%s&time=%s", key, timestamp)

	fmt.Printf("生成的 Key: %s\n", key)
	fmt.Printf("生成的时间戳: %s\n", timestamp)
	fmt.Printf("请求的 URL: %s\n\n", apiURL)

	// 发起 HTTP GET 请求
	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 读取并打印响应体
	fmt.Println("--- API 响应 ---")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return
	}

	fmt.Println(string(body))
}
