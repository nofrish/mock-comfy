package main

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"math/rand"
	"net/http"
	"sync"
	"time"
	"fmt"
	"os"
	"io"
	"path/filepath"
)

type PromptInfo struct {
	Prompt   map[string]interface{}
	ClientID string
	Status   string
	Output   map[string]interface{}
	ID       int
	PromptID string // 新增字段
}

type ComfyUIMock struct {
	prompts     map[string]*PromptInfo
	queueID     int
	runningTask *PromptInfo
	mu          sync.Mutex
}

func NewComfyUIMock() *ComfyUIMock {
	return &ComfyUIMock{
		prompts: make(map[string]*PromptInfo),
		queueID: 0,
	}
}

func main() {
	mock := NewComfyUIMock()

	r := gin.Default()

	r.POST("/prompt", mock.handlePrompt)
	r.GET("/history/:prompt_id", mock.handleHistory)
	r.GET("/queue", mock.handleQueue)

	r.Run(":8288")
}

func (m *ComfyUIMock) handlePrompt(c *gin.Context) {
	var request struct {
		ClientID string                 `json:"client_id"`
		Prompt   map[string]interface{} `json:"prompt"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	promptID := generatePromptID()

	m.mu.Lock()
	m.queueID++
	promptInfo := &PromptInfo{
		Prompt:   request.Prompt,
		ClientID: request.ClientID,
		Status:   "pending",
		ID:       m.queueID,
		PromptID: promptID, // 设置 PromptID
	}
	m.prompts[promptID] = promptInfo
	m.mu.Unlock()

	go m.processQueue()

	c.JSON(http.StatusOK, gin.H{"prompt_id": promptID})
}

func (m *ComfyUIMock) handleHistory(c *gin.Context) {
	promptID := c.Param("prompt_id")

	m.mu.Lock()
	prompt, exists := m.prompts[promptID]
	m.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Prompt not found"})
		return
	}

	if prompt.Status != "completed" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		promptID: gin.H{
			"prompt":  prompt.Prompt,
			"outputs": prompt.Output,
			"status": gin.H{
				"status_str": "success",
				"completed":  true,
				"messages": []interface{}{
					[]interface{}{"execution_start", gin.H{"prompt_id": promptID}},
					[]interface{}{"execution_cached", gin.H{"nodes": []string{"4", "7", "5", "6"}, "prompt_id": promptID}},
				},
			},
		},
	})
}

func (m *ComfyUIMock) handleQueue(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queueRunning := []interface{}{}
	queuePending := []interface{}{}

	if m.runningTask != nil {
		queueRunning = append(queueRunning, []interface{}{
			m.runningTask.ID,
			m.runningTask.PromptID,
			m.runningTask.Prompt,
			[]string{"9"},
		})
	}

	for _, prompt := range m.prompts {
		if prompt.Status == "pending" {
			queuePending = append(queuePending, []interface{}{
				prompt.ID,
				prompt.PromptID,
				prompt.Prompt,
				[]string{"9"},
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"queue_running": queueRunning,
		"queue_pending": queuePending,
	})
}

func (m *ComfyUIMock) processQueue() {
	m.mu.Lock()
	if m.runningTask != nil {
		m.mu.Unlock()
		return
	}

	for _, prompt := range m.prompts {
		if prompt.Status == "pending" {
			m.runningTask = prompt
			prompt.Status = "processing"
			break
		}
	}
	m.mu.Unlock()

	if m.runningTask != nil {
		m.processPrompt(m.runningTask)
		m.mu.Lock()
		m.runningTask = nil
		m.mu.Unlock()
		go m.processQueue()
	}
}

func (m *ComfyUIMock) processPrompt(prompt *PromptInfo) {
	// 模拟处理时间，随机 10-20 秒
	processingTime := 10 + rand.Intn(11)
	time.Sleep(time.Duration(processingTime) * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	prompt.Status = "completed"
	prompt.Output = generateMockOutput(prompt.PromptID)

	// 复制图片文件并重命名
	err := copyAndRenameImage(prompt.PromptID)
	if err != nil {
		fmt.Printf("复制和重命名图片时出错: %v\n", err)
	}
}

func generateMockOutput(promptID string) map[string]interface{} {
	return map[string]interface{}{
		"9": map[string]interface{}{
			"images": []map[string]interface{}{
				{
					"filename":  fmt.Sprintf("%s.png", promptID[:8]),
					"subfolder": "",
					"type":      "output",
				},
			},
		},
	}
}

func generatePromptID() string {
	return uuid.New().String()
}

func copyAndRenameImage(promptID string) error {
	sourcePath := "resources/image.jpg"
	outputDir := "outputs"
	newFileName := "output_" + promptID[:8] + ".jpg"
	destPath := filepath.Join(outputDir, newFileName)

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 打开源文件
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer destFile.Close()

	// 复制文件内容
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	return nil
}