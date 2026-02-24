package role

type RoleDataScopeRuleInput struct {
	Domain      string `json:"domain"`
	ResourceKey string `json:"resourceKey"`
	ScopeType   string `json:"scopeType"`
	ScopeValue  string `json:"scopeValue,omitempty"`
}

type RoleDataScopeRule struct {
	Domain      string `json:"domain"`
	ResourceKey string `json:"resourceKey"`
	ScopeType   string `json:"scopeType"`
	ScopeValue  string `json:"scopeValue,omitempty"`
}

type DeleteRoleDataScopeRuleRequest struct {
	Domain      string `json:"domain"`
	ResourceKey string `json:"resourceKey"`
}
