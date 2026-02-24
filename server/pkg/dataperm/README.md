# 数据权限过滤器使用指南

## 快速开始

### 1. 导入包

```go
import "leeforge-backend/pkg/dataperm"
```

### 2. 在 Service 中使用

#### 方式一：使用全局方法（推荐）

```go
// internal/modules/user/invitation_service.go

// 查询列表 - 自动过滤
func (s *InvitationService) List(ctx context.Context) ([]*ent.InvitationToken, error) {
    query := s.client.InvitationToken.Query()

    // 一行代码处理数据权限
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.All(ctx)
}

// 创建 - 不需要手动设置 created_by_id（由 Hook 自动处理）
func (s *InvitationService) Create(ctx context.Context, email string) (*ent.InvitationToken, error) {
    return s.client.InvitationToken.
        Create().
        SetToken(generateToken()).
        SetEmail(email).
        Save(ctx)  // created_by_id 由 DataPermissionHook 自动设置
}

// 更新 - 自动设置 updated_by_id
func (s *InvitationService) Update(ctx context.Context, id uuid.UUID, email string) (*ent.InvitationToken, error) {
    query := s.client.InvitationToken.UpdateOneID(id)

    // 非管理员只能更新自己创建的
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.
        SetEmail(email).
        Save(ctx)  // updated_by_id 自动设置
}

// 删除 - 自动设置 deleted_by_id
func (s *InvitationService) Delete(ctx context.Context, id uuid.UUID) error {
    query := s.client.InvitationToken.UpdateOneID(id)

    // 非管理员只能删除自己创建的
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.
        SetDeletedAt(time.Now()).
        Exec(ctx)  // deleted_by_id 自动设置
}

// 根据权限返回不同数据
func (s *InvitationService) GetStatistics(ctx context.Context) (*Statistics, error) {
    if dataperm.IsAdmin(ctx) {
        // 管理员看到所有统计
        return s.getAllStatistics(ctx)
    }

    // 普通用户只看到自己的
    userID := dataperm.GetCreatorID(ctx)
    return s.getUserStatistics(ctx, userID)
}
```

#### 方式二：使用自定义过滤器实例

```go
type InvitationService struct {
    client *ent.Client
    filter *dataperm.Filter  // 自定义过滤器
}

func NewInvitationService(client *ent.Client) *InvitationService {
    return &InvitationService{
        client: client,
        filter: dataperm.NewFilter().
            WithSuperAdminRoles([]string{"super_admin", "admin", "god"}).
            WithBypassRoles([]string{"system", "service"}),
    }
}

func (s *InvitationService) List(ctx context.Context) ([]*ent.InvitationToken, error) {
    query := s.client.InvitationToken.Query()

    // 使用自定义过滤器
    if needFilter, userID := s.filter.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.All(ctx)
}
```

## API 说明

### 全局方法

#### `dataperm.ShouldFilterByCreator(ctx) (bool, uuid.UUID)`

判断是否需要按创建者过滤数据。

- **返回**：`(needFilter bool, creatorID uuid.UUID)`
- **使用场景**：查询、更新、删除时判断是否需要添加 `created_by_id` 条件

```go
// 示例
if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
    query = query.Where(entity.CreatedByID(userID))
}
```

#### `dataperm.GetCreatorID(ctx) uuid.UUID`

获取当前用户ID。

- **返回**：`uuid.UUID`（如果没有身份信息返回 `uuid.Nil`）
- **使用场景**：需要显式获取当前用户ID时

```go
// 示例
userID := dataperm.GetCreatorID(ctx)
if userID != uuid.Nil {
    // 使用 userID
}
```

#### `dataperm.IsAdmin(ctx) bool`

判断当前用户是否是管理员。

- **返回**：`bool`
- **使用场景**：根据权限返回不同数据或执行不同逻辑

```go
// 示例
if dataperm.IsAdmin(ctx) {
    // 管理员逻辑
} else {
    // 普通用户逻辑
}
```

#### `dataperm.HasRole(ctx, role string) bool`

判断当前用户是否有指定角色。

- **参数**：`role string` - 角色名称
- **返回**：`bool`
- **使用场景**：细粒度的权限控制

```go
// 示例
if dataperm.HasRole(ctx, "editor") {
    // 编辑者可以执行的操作
}
```

## 配置说明

### 默认配置

- **超级管理员角色**：`["super_admin", "admin"]`
- **绕过角色**：`[]`（空）

### 自定义配置

```go
filter := dataperm.NewFilter().
    WithSuperAdminRoles([]string{"super_admin", "admin", "god"}).
    WithBypassRoles([]string{"system", "service"})
```

## 权限规则

| 用户类型 | 查询 | 创建 | 更新 | 删除 |
|---------|------|------|------|------|
| **超级管理员** | 所有数据 | ✅ | 所有数据 | 所有数据 |
| **普通用户** | 自己创建的 | ✅ | 自己创建的 | 自己创建的 |
| **系统调用（无身份）** | 所有数据 | ✅ | 所有数据 | 所有数据 |

## 完整示例

```go
package invitation

import (
    "context"
    "time"

    "github.com/google/uuid"

    "leeforge-backend/ent"
    "leeforge-backend/ent/invitationtoken"
    "leeforge-backend/pkg/dataperm"
)

type Service struct {
    client *ent.Client
}

func NewService(client *ent.Client) *Service {
    return &Service{client: client}
}

// List 查询列表
func (s *Service) List(ctx context.Context) ([]*ent.InvitationToken, error) {
    query := s.client.InvitationToken.Query()

    // 数据权限过滤
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.
        Order(ent.Desc(invitationtoken.FieldCreatedAt)).
        All(ctx)
}

// Get 查询单个
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*ent.InvitationToken, error) {
    query := s.client.InvitationToken.Query().
        Where(invitationtoken.ID(id))

    // 数据权限过滤
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    return query.Only(ctx)
}

// Create 创建
func (s *Service) Create(ctx context.Context, email string) (*ent.InvitationToken, error) {
    // created_by_id 由 DataPermissionHook 自动设置
    return s.client.InvitationToken.
        Create().
        SetToken(generateToken()).
        SetEmail(email).
        SetExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
        Save(ctx)
}

// Update 更新
func (s *Service) Update(ctx context.Context, id uuid.UUID, email string) (*ent.InvitationToken, error) {
    query := s.client.InvitationToken.UpdateOneID(id)

    // 数据权限过滤
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    // updated_by_id 由 DataPermissionHook 自动设置
    return query.
        SetEmail(email).
        Save(ctx)
}

// Delete 软删除
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
    query := s.client.InvitationToken.UpdateOneID(id)

    // 数据权限过滤
    if needFilter, userID := dataperm.ShouldFilterByCreator(ctx); needFilter {
        query = query.Where(invitationtoken.CreatedByID(userID))
    }

    // deleted_by_id 由 DataPermissionHook 自动设置
    return query.
        SetDeletedAt(time.Now()).
        Exec(ctx)
}

// Statistics 统计数据
func (s *Service) Statistics(ctx context.Context) (int, error) {
    query := s.client.InvitationToken.Query()

    // 根据权限返回不同的统计
    if dataperm.IsAdmin(ctx) {
        // 管理员看到所有
        return query.Count(ctx)
    }

    // 普通用户只看到自己的
    userID := dataperm.GetCreatorID(ctx)
    return query.
        Where(invitationtoken.CreatedByID(userID)).
        Count(ctx)
}

func generateToken() string {
    // 生成随机 token
    return uuid.New().String()
}
```

## 注意事项

1. **Context 必须包含 Identity**：通过认证中间件自动注入
2. **Hook 必须注册**：在 Ent Client 初始化时注册 `DataPermissionHook`
3. **手动设置优先**：如果手动调用 `SetCreatedByID()`，会覆盖自动设置
4. **系统调用不需要过滤**：后台任务、定时任务等可以不包含 Identity
5. **查询性能**：添加 `created_by_id` 索引以提升查询性能

## 与 Hook 的关系

- **Hook**（`DataPermissionHook`）：自动设置审计字段（创建/更新/删除时）
- **Filter**（`dataperm` 包）：判断是否需要过滤查询（查询时）

两者配合使用，实现完整的数据权限控制：
- **写操作**：Hook 自动设置 `created_by_id`、`updated_by_id`、`deleted_by_id`
- **读操作**：Filter 帮助判断是否需要过滤，Service 层根据结果添加 Where 条件
