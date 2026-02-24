package user

type CreateInvitationAPIRequest struct {
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	DomainType string   `json:"domainType"`
	DomainKey  string   `json:"domainKey"`
	RoleIDs    []string `json:"roleIds"`
}

type ActivateInvitationAPIRequest struct {
	InviteJWT       string `json:"inviteJwt"`
	Nickname        string `json:"nickname"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}
