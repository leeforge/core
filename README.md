# Leeforge Core - Backend Runtime Framework

`leeforge/core` 是 Headless CMS 后端的**运行时引导框架**，提供一键式初始化、模块系统、插件机制和多框架适配。

## 🎯 核心职责

- **运行时初始化** - 通过 `BuildRuntime()` 一键创建完整的后端引擎
- **框架适配** - 原生支持 Chi/Gin/Echo 三种路由框架
- **模块化架构** - 垂直切片（Vertical Slice）模块注册与启动
- **插件系统** - 通过 `PluginRegistrar` 回调注册第三方插件
- **数据库管理** - 集成 Ent ORM 和自动化迁移
- **文档生成** - 自动生成并暴露 Swagger API 文档

## 📦 快速开始

### 基础使用（Gin 框架）

```go
package bootstrap

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/leeforge/core"
)

func NewApp() (*gin.Engine, error) {
	// 1. 创建运行时选项
	opts := core.RuntimeOptions{
		ConfigPath:      "./configs",
		PluginRegistrar: registerPlugins, // 可选：注册插件
	}

	// 2. 构建运行时
	rt, err := core.BuildRuntime(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	// 3. 创建 Gin 引擎并注册路由
	engine := gin.Default()
	engine.Any("/*any", gin.WrapH(rt.Handler()))

	return engine, nil
}

// 4. 启动服务
func main() {
	engine, _ := NewApp()
	engine.Run(":8080")
}
```

详见 `server/examples/internal/bootstrap/app.go`。

## 🔧 运行时配置

### RuntimeOptions

```go
type RuntimeOptions struct {
	ConfigPath       string                       // 配置文件目录路径
	ModuleFactories  []corecore.ModuleFactory     // 外部模块工厂列表（可选）
	ResourceProvider ResourceProvider              // 数据库和资源提供者（可选）
	SkipPlugins      bool                         // 跳过插件加载（开发/测试时常用）
	SkipMigrate      bool                         // 跳过数据库迁移
	BasePath         string                       // API 基路径（默认：/api/v1）
	Logger           *zap.Logger                  // 日志实例（可选）
	PluginRegistrar  PluginRegistrar              // 插件注册回调函数
}
```

### 简化初始化

**跳过数据库（用于测试）**：
```go
opts := core.RuntimeOptions{
	ConfigPath:       "./configs",
	ResourceProvider: &noopResourceProvider{},
	SkipPlugins:      true,
	SkipMigrate:      true,
}
```

**使用默认资源提供者（生产环境）**：
```go
opts := core.RuntimeOptions{
	ConfigPath: "./configs",
	// ResourceProvider 不指定则使用默认实现
}
```

## 🌐 框架适配

### Chi 框架（推荐）

```go
router, _ := core.BuildRuntime(ctx, opts)
chiRouter := router.Router()
http.ListenAndServe(":8080", chiRouter)
```

### Gin 框架

```go
rt, _ := core.BuildRuntime(ctx, opts)
engine := gin.Default()
engine.Any("/*any", gin.WrapH(rt.Handler()))
engine.Run(":8080")
```

### Echo 框架

```go
rt, _ := core.BuildRuntime(ctx, opts)
e := echo.New()
e.Any("/*", echo.WrapHandler(rt.Handler()))
e.Start(":8080")
```

## 📝 API 端点

### 健康检查（内置）

```
GET /api/v1/health
```

响应：
```json
{
  "status": "ok"
}
```

### Swagger 文档（自动生成）

```
GET /api/v1/swagger/index.html    # Swagger UI
GET /api/v1/swagger/doc.json      # OpenAPI JSON
GET /api/v1/swagger/doc.yaml      # OpenAPI YAML
```

## 🔌 插件系统

### 定义插件注册回调

```go
func registerExamplePlugins(
	rt *frameworkruntime.Runtime,
	services *frameplugin.ServiceRegistry,
	logger *zap.Logger,
) error {
	// 注册自定义服务、中间件等
	return nil
}
```

### 在运行时中使用

```go
opts := core.RuntimeOptions{
	ConfigPath:      "./configs",
	PluginRegistrar: registerExamplePlugins,
}

rt, _ := core.BuildRuntime(ctx, opts)
```

## 📂 目录结构

```
server/core/
├── runtime.go              # 核心运行时定义
├── domain.go              # 领域模型接口
├── host/                  # Chi 框架适配
│   ├── register_chi.go    # Chi 路由注册
│   └── route_conflict.go  # 路由冲突检测
├── adapters/
│   ├── gin.go             # Gin 框架适配
│   └── echo.go            # Echo 框架适配
├── bootstrap/             # 启动相关工具
├── modules/               # 模块接口定义
├── middleware/            # HTTP 中间件
├── server/                # 内部服务实现
├── ent/                   # Ent ORM Schema
└── examples/              # 使用示例
```

## 🚀 完整示例

参考 `server/examples/` 目录：

```bash
# 启动示例服务
cd server/examples
go run ./cmd/server

# 访问 Swagger
open http://localhost:8080/swagger/index.html

# 生成 Swagger 文档
make swagger

# 运行测试（包括框架兼容性检查）
make test
```

## 🔍 关键接口

### Runtime 接口

```go
type Runtime interface {
	Router() chi.Router        // 获取 Chi 路由实例
	Handler() http.Handler     // 获取 HTTP 处理器（用于适配其他框架）
	Shutdown(context.Context) error // 优雅关闭
}
```

### ResourceProvider 接口

用于为运行时提供数据库客户端、迁移函数等资源：

```go
type ResourceProvider interface {
	Build(context.Context, ResourceInput) (*RuntimeResources, error)
}

type RuntimeResources struct {
	CoreClient      *ent.Client                    // Ent 数据库客户端
	MigrationRunner func(context.Context) error    // 数据库迁移函数
	Closers         []io.Closer                    // 需要关闭的资源
}
```

## 🛠️ 开发工作流

### 添加新模块

1. 在 `server/core/modules/` 中实现 `ModuleFactory` 工厂函数，返回实现 `Module` 接口的结构体
2. 在 `RuntimeOptions.ModuleFactories` 中注册该工厂函数
3. 运行时自动启动模块的 HTTP 处理器

#### ModuleFactory 注册示例

```go
// 1. 创建模块实现
type MyCustomModule struct {
    logger frameLogging.Logger
    deps   *corecore.Dependencies
}

func (m *MyCustomModule) Name() string { return "my-custom-module" }

func (m *MyCustomModule) RegisterPublicRoutes(router chi.Router) {
    router.Get("/public-endpoint", /* handler */)
}

func (m *MyCustomModule) RegisterPrivateRoutes(router chi.Router) {
    router.Get("/private-endpoint", /* handler */)
}

// 2. 创建工厂函数
func NewMyCustomModule(logger frameLogging.Logger, deps *corecore.Dependencies) corecore.Module {
    return &MyCustomModule{
        logger: logger,
        deps:   deps,
    }
}

// 3. 在 RuntimeOptions 中注册
opts := core.RuntimeOptions{
    ConfigPath:      "./configs",
    ModuleFactories: []corecore.ModuleFactory{
        NewMyCustomModule,  // 注册你的工厂函数
    },
}

rt, _ := core.BuildRuntime(context.Background(), opts)
```

### 添加自定义中间件

1. 实现 `http.Handler` 或 `chi.Router`
2. 在 `host/register_chi.go` 中注册
3. 所有请求自动应用

### 生成 API 文档

```bash
cd server/examples
make swagger  # 生成 /docs/swagger.{json,yaml}
```

## 📖 配置文件

配置文件位于 `configs/` 目录，支持 YAML 格式：

```yaml
# configs/config.yaml
database:
  driver: postgres
  dsn: postgres://user:pass@localhost/dbname

server:
  port: 8080
  basePath: /api/v1

plugins:
  enabled:
    - plugin-name
```

## ✅ 单元测试

```go
func TestNewApp(t *testing.T) {
	app, _ := bootstrap.NewAppForTest()
	assert.NotNil(t, app)
}
```

使用 `NewAppForTest()` 跳过数据库和插件，加速测试。

## 📚 相关文档

- **框架层**：`server/framework/` - 通用插件系统和配置管理
- **示例项目**：`server/examples/` - 完整的使用示例
- **业务后端**：`server/backend/` - CMS 具体业务实现
