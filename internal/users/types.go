package users

import "time"

// User is the public profile returned by the API (never includes password hash).
type User struct {
	ID              int64     `json:"id"`
	Phone           string    `json:"phone"`
	DisplayName     string    `json:"displayName"`
	AvatarURL       string    `json:"avatarUrl"`
	Email           string    `json:"email"`
	WechatPushToken string    `json:"wechatPushToken"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// UpdateProfileInput updates mutable profile fields.
type UpdateProfileInput struct {
	DisplayName     *string `json:"displayName"`
	AvatarURL       *string `json:"avatarUrl"`
	Email           *string `json:"email"`
	WechatPushToken *string `json:"wechatPushToken"`
}

// LoginResult is returned after successful authentication.
type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      User      `json:"user"`
}

type userRow struct {
	ID              int64
	Phone           string
	PasswordHash    string
	DisplayName     string
	AvatarURL       string
	Email           string
	WechatPushToken string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r userRow) toPublic() User {
	return User{
		ID:              r.ID,
		Phone:           r.Phone,
		DisplayName:     r.DisplayName,
		AvatarURL:       r.AvatarURL,
		Email:           r.Email,
		WechatPushToken: r.WechatPushToken,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
