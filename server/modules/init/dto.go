package initmod

// InitStatusResponse 初始化状态响应
type InitStatusResponse struct {
	Initialized bool   `json:"initialized"`
	Version     string `json:"version"`
}

// AdminInput 管理员账号输入
type AdminInput struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Nickname string `json:"nickname" validate:"required"`
}

// InitSetupRequest 初始化请求
type InitSetupRequest struct {
	InitKey string     `json:"initKey" validate:"required"`
	Admin   AdminInput `json:"admin" validate:"required"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// InitSetupResponse 初始化响应
type InitSetupResponse struct {
	AdminUser          *UserInfo `json:"adminUser"`
	RolesCreated       int       `json:"rolesCreated"`
	PermissionsCreated int       `json:"permissionsCreated"`
	Message            string    `json:"message"`
}
