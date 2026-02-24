package user

type ActivatePasswordResetAPIRequest struct {
	ResetJWT        string `json:"resetJwt"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}
