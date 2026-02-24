package permission

// ListPermissionsOptions represents the query options for listing permissions.
type ListPermissionsOptions struct {
	Page     int
	PageSize int
	Scope    string
	Status   string
	Keyword  string
}

// SyncEndpointsResult represents the result of a sync operation.
type SyncEndpointsResult struct {
	TotalRoutes   int      `json:"totalRoutes"`
	Created       int      `json:"created"`
	Updated       int      `json:"updated"`
	Deleted       int      `json:"deleted"`
	CreatedRoutes []string `json:"createdRoutes,omitempty"`
	UpdatedRoutes []string `json:"updatedRoutes,omitempty"`
	DeletedRoutes []string `json:"deletedRoutes,omitempty"`
}

// SyncStatus represents the current sync status.
type SyncStatus struct {
	TotalEndpoints int            `json:"totalEndpoints"`
	ByMethod       map[string]int `json:"byMethod"`
	Endpoints      []string       `json:"endpoints"`
}
