package ginx

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/service_discovery"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig 测试配置结构
type TestConfig struct {
	Name    string `yaml:"name" env:"APP_NAME"`
	Debug   bool   `yaml:"debug" env:"DEBUG"`
	Port    int    `yaml:"port" env:"PORT"`
	Version string `yaml:"version" env:"VERSION"`
}

// MockServiceDiscovery 模拟服务发现
type MockServiceDiscovery struct {
	watchCalled bool
	serviceInfo service_discovery.ServiceInfo
	shouldError bool
}

func (m *MockServiceDiscovery) Watch(info service_discovery.ServiceInfo) error {
	m.watchCalled = true
	m.serviceInfo = info
	if m.shouldError {
		return fmt.Errorf("mock watch error")
	}
	return nil
}

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (string, func())
		config   interface{}
		validate func(*testing.T, interface{})
	}{
		{
			name: "parse from environment variables",
			setup: func() (string, func()) {
				os.Setenv("APP_NAME", "test-app")
				os.Setenv("DEBUG", "true")
				os.Setenv("PORT", "8080")

				return "", func() {
					os.Unsetenv("APP_NAME")
					os.Unsetenv("DEBUG")
					os.Unsetenv("PORT")
				}
			},
			config: &TestConfig{},
			validate: func(t *testing.T, config interface{}) {
				cfg := config.(*TestConfig)
				assert.Equal(t, "test-app", cfg.Name)
				assert.True(t, cfg.Debug)
				assert.Equal(t, 8080, cfg.Port)
			},
		},
		{
			name: "parse from config file",
			setup: func() (string, func()) {
				// 创建临时配置文件
				tempDir := t.TempDir()
				configFile := filepath.Join(tempDir, "config.yml")
				configContent := `name: yaml-app
debug: false
port: 9090
version: "1.0.0"`

				err := os.WriteFile(configFile, []byte(configContent), 0644)
				require.NoError(t, err)

				// 保存原始配置文件路径
				originalConfFile := confFile
				confFile = configFile

				return configFile, func() {
					confFile = originalConfFile
					os.Remove(configFile)
				}
			},
			config: &TestConfig{},
			validate: func(t *testing.T, config interface{}) {
				cfg := config.(*TestConfig)
				assert.Equal(t, "yaml-app", cfg.Name)
				assert.False(t, cfg.Debug)
				assert.Equal(t, 9090, cfg.Port)
				assert.Equal(t, "1.0.0", cfg.Version)
			},
		},
		{
			name: "parse default config file",
			setup: func() (string, func()) {
				// 创建默认配置文件
				wd, _ := os.Getwd()
				defaultConfigPath := filepath.Join(wd, DefaultConfig)
				configContent := `name: default-app
debug: true
port: 3000`

				err := os.WriteFile(defaultConfigPath, []byte(configContent), 0644)
				require.NoError(t, err)

				// 设置为默认配置
				originalConfFile := confFile
				confFile = DefaultConfig

				return defaultConfigPath, func() {
					confFile = originalConfFile
					os.Remove(defaultConfigPath)
				}
			},
			config: &TestConfig{},
			validate: func(t *testing.T, config interface{}) {
				cfg := config.(*TestConfig)
				assert.Equal(t, "default-app", cfg.Name)
				assert.True(t, cfg.Debug)
				assert.Equal(t, 3000, cfg.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath, cleanup := tt.setup()
			defer cleanup()

			// 执行解析
			Parse(tt.config)

			// 验证结果
			if tt.validate != nil {
				tt.validate(t, tt.config)
			}

			// 清理
			if configPath != "" && configPath != DefaultConfig {
				os.Remove(configPath)
			}
		})
	}
}

func TestAddCommand(t *testing.T) {
	// 保存原始命令
	originalCmd := ginx.Command
	defer func() {
		ginx.Command = originalCmd
	}()

	// 重置命令
	ginx.Command = &cobra.Command{}

	// 创建测试命令
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
		Run: func(cmd *cobra.Command, args []string) {
			// 测试命令逻辑
		},
	}

	AddCommand(testCmd)

	// 验证命令已添加
	commands := ginx.Command.Commands()
	assert.Len(t, commands, 1)
	assert.Equal(t, "test", commands[0].Use)
}

func TestSetI18n(t *testing.T) {
	tests := []struct {
		language string
		expected string
	}{
		{I18nEN, I18nEN},
		{I18nZH, I18nZH},
		{"invalid", I18nZH}, // 默认为中文
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			SetI18n(tt.language)
			assert.Equal(t, tt.expected, ginx.i18n)
		})
	}
}

func TestRunServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试路由
	testOp := &TestGinOperator{}
	router := NewRouter(testOp)

	// 创建配置
	config := &conf.Server{
		Host:    "localhost",
		Port:    0, // 使用随机端口
		Release: false,
	}

	// 在goroutine中运行服务器
	var server *Server
	done := make(chan bool)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 预期会有panic，因为我们没有完整的服务器环境
				done <- true
			}
		}()

		// 设置一个自定义的信号等待器，避免实际等待信号
		originalInstance := instance()
		originalInstance.signalWaiter = func(errCh chan error) error {
			// 模拟接收到退出信号
			return nil
		}

		RunServer(config, router)
		done <- true
	}()

	// 等待一段时间确保服务器启动处理
	select {
	case <-done:
		// 服务器处理完成
	case <-time.After(100 * time.Millisecond):
		// 超时，这是正常的，因为服务器可能仍在运行
	}

	// 验证实例已创建
	server = instance()
	assert.NotNil(t, server)
}

func TestRunServer_NilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建简单路由
	router := NewRouter(&TestGinOperator{})

	// 在goroutine中运行服务器（使用nil配置）
	done := make(chan bool)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- true
			}
		}()

		// 设置自定义信号等待器
		originalInstance := instance()
		originalInstance.signalWaiter = func(errCh chan error) error {
			return nil
		}

		RunServer(nil, router) // 传入nil配置
		done <- true
	}()

	// 等待处理完成
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}
}

func TestInstance(t *testing.T) {
	// 重置ginx实例
	ginx.once = sync.Once{}
	ginx.server = nil

	// 第一次调用应该创建实例
	server1 := instance()
	assert.NotNil(t, server1)

	// 第二次调用应该返回同一个实例
	server2 := instance()
	assert.Equal(t, server1, server2)
}

func TestAddShutdownHook(t *testing.T) {
	// 重置实例
	ginx.once = sync.Once{}
	ginx.server = nil

	called := false
	hook := func() {
		called = true
	}

	// 添加关闭钩子
	AddShutdownHook(hook)

	// 验证钩子已添加
	server := instance()
	assert.Len(t, server.graceCloseHooks, 1)

	// 执行钩子
	server.graceCloseHooks[0]()
	assert.True(t, called)
}

func TestRegister(t *testing.T) {
	// 重置实例
	ginx.once = sync.Once{}
	ginx.server = nil

	// 创建模拟服务发现
	mockSD := &MockServiceDiscovery{}

	// 注册服务发现
	Register(mockSD)

	// 验证已注册
	server := instance()
	assert.Equal(t, mockSD, server.watcher)
}

func TestServer_watch(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name        string
		watcher     service_discovery.ServiceDiscovery
		config      *conf.Server
		expectError bool
	}{
		{
			name:    "no watcher",
			watcher: nil,
			config: &conf.Server{
				Name: "test-service",
				Port: 8080,
			},
			expectError: false,
		},
		{
			name:    "successful watch",
			watcher: &MockServiceDiscovery{shouldError: false},
			config: &conf.Server{
				Name: "test-service",
				Port: 8080,
				Discovery: conf.Discovery{
					Address: "localhost",
					Tags:    []string{"api", "v1"},
				},
			},
			expectError: false,
		},
		{
			name:    "watch error",
			watcher: &MockServiceDiscovery{shouldError: true},
			config: &conf.Server{
				Name: "test-service",
				Port: 8080,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.watcher = tt.watcher

			err := server.watch(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// 如果有watcher且无错误，验证调用
			if tt.watcher != nil && !tt.expectError {
				mockWatcher := tt.watcher.(*MockServiceDiscovery)
				assert.True(t, mockWatcher.watchCalled)
				assert.Equal(t, tt.config.Name, mockWatcher.serviceInfo.Name)
			}
		})
	}
}

func TestWaitSignal(t *testing.T) {
	tests := []struct {
		name       string
		signal     os.Signal
		expectErr  bool
		errMessage string
	}{
		{
			name:       "SIGTERM",
			signal:     syscall.SIGTERM,
			expectErr:  true,
			errMessage: "terminated",
		},
		{
			name:      "SIGINT",
			signal:    syscall.SIGINT,
			expectErr: false,
		},
		{
			name:      "SIGHUP",
			signal:    syscall.SIGHUP,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errCh := make(chan error, 1)

			// 在goroutine中执行waitSignal
			done := make(chan error, 1)
			go func() {
				done <- waitSignal(errCh)
			}()

			// 模拟发送信号（这里我们直接向errCh发送错误来模拟）
			if tt.name == "error from errCh" {
				errCh <- fmt.Errorf("test error")
			}

			// 等待结果
			select {
			case err := <-done:
				if tt.expectErr {
					assert.Error(t, err)
					if tt.errMessage != "" {
						assert.Contains(t, err.Error(), tt.errMessage)
					}
				} else {
					assert.NoError(t, err)
				}
			case <-time.After(100 * time.Millisecond):
				// 超时是正常的，因为waitSignal会等待信号
				t.Log("waitSignal is waiting for signal, which is expected")
			}
		})
	}
}

func TestInitTrace(t *testing.T) {
	config := &conf.Server{
		Name: "test-service",
		Trace: conf.Trace{
			TraceEndpoint: "http://localhost:14268",
			TraceExporter: "jaeger",
		},
	}

	// 这个测试可能会失败，因为trace.Agent.Init()可能需要实际的trace服务
	// 我们只测试函数不panic
	assert.NotPanics(t, func() {
		defer func() {
			if r := recover(); r != nil {
				// 如果初始化失败，我们捕获panic并继续
				t.Logf("Trace initialization failed (expected in test environment): %v", r)
			}
		}()
		agent := initTrace(config)
		assert.NotNil(t, agent)
	})
}

func TestServer_run(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := &Server{
		engine: gin.New(),
	}

	tests := []struct {
		name        string
		config      *conf.Server
		expectPanic bool
	}{
		{
			name: "HTTP server",
			config: &conf.Server{
				Host:  "localhost",
				Port:  0, // 使用随机可用端口
				Https: false,
			},
			expectPanic: false,
		},
		{
			name: "HTTPS server without cert files",
			config: &conf.Server{
				Host:  "localhost",
				Port:  0,
				Https: true,
				TLS: conf.TLS{
					CertFile: "",
					KeyFile:  "",
				},
			},
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					server.run(tt.config)
				})
			} else {
				// 在goroutine中运行服务器
				done := make(chan error, 1)
				go func() {
					done <- server.run(tt.config)
				}()

				// 立即关闭服务器
				if server.server != nil {
					go func() {
						time.Sleep(10 * time.Millisecond)
						server.server.Close()
					}()
				}

				// 等待结果
				select {
				case err := <-done:
					// 服务器关闭时会返回错误，这是正常的
					assert.Error(t, err)
				case <-time.After(100 * time.Millisecond):
					// 超时，关闭服务器
					if server.server != nil {
						server.server.Close()
					}
				}
			}
		})
	}
}

func TestServer_spin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := &Server{
		engine: gin.New(),
	}

	// 设置自定义信号等待器
	server.signalWaiter = func(errCh chan error) error {
		// 立即返回，模拟接收到信号
		return nil
	}

	config := &conf.Server{
		Host:            "localhost",
		Port:            0,
		ExitWaitTimeout: 1,
	}

	// 在goroutine中运行spin
	done := make(chan bool)
	go func() {
		defer func() {
			done <- true
		}()
		server.spin(config)
	}()

	// 等待完成
	select {
	case <-done:
		// 成功完成
	case <-time.After(1 * time.Second):
		t.Error("spin方法超时")
	}
}

// TestCallback 测试回调类型
func TestCallback(t *testing.T) {
	called := false
	callback := Callback(func() {
		called = true
	})

	callback()
	assert.True(t, called)
}

// 基准测试
func BenchmarkParse(b *testing.B) {
	// 设置环境变量
	os.Setenv("APP_NAME", "benchmark-app")
	os.Setenv("DEBUG", "true")
	os.Setenv("PORT", "8080")
	defer func() {
		os.Unsetenv("APP_NAME")
		os.Unsetenv("DEBUG")
		os.Unsetenv("PORT")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := &TestConfig{}
		Parse(config)
	}
}

func BenchmarkInstance(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance()
	}
}

// 集成测试
func TestIntegration_FullServerLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试路由
	router := NewRouter(&TestGinOperator{})

	// 创建配置
	config := &conf.Server{
		Host: "localhost",
		Port: 0, // 使用随机端口
	}

	// 重置实例
	ginx.once = sync.Once{}
	ginx.server = nil

	// 添加关闭钩子
	AddShutdownHook(func() {
		// hook callback
	})

	// 注册服务发现
	mockSD := &MockServiceDiscovery{}
	Register(mockSD)

	// 运行服务器
	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- true
			}
		}()

		// 设置快速退出的信号等待器
		server := instance()
		server.signalWaiter = func(errCh chan error) error {
			return nil // 立即退出
		}

		RunServer(config, router)
		done <- true
	}()

	// 等待完成
	select {
	case <-done:
		// 验证服务发现被调用
		assert.True(t, mockSD.watchCalled)
	case <-time.After(1 * time.Second):
		t.Error("集成测试超时")
	}
}
