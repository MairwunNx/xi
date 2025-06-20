package entities

type UserRight string

const (
	UserRightSwitchMode  UserRight = "switch_mode"
	UserRightEditMode    UserRight = "edit_mode"
	UserRightManageUsers UserRight = "manage_users"
)
