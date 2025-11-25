package repository

import (
	"errors"
	"slices"
	"strings"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"
)

var ErrNothingChanged = errors.New("nothing changed")
var ErrRightNotFound = errors.New("right not found")

var AvailableRights = []string{"switch_mode", "edit_mode", "manage_users", "manage_context", "manage_tariffs"}

type RightsRepository struct {
	users *UsersRepository
}

func NewRightsRepository(users *UsersRepository) *RightsRepository {
	return &RightsRepository{users: users}
}

func (x *RightsRepository) IsUserHasRight(logger *tracing.Logger, user *entities.User, scope string) bool {
	defer tracing.ProfilePoint(logger, "Rights is user has right completed", "repository.rights.is.user.has.right", "user_id", user.ID, "scope", scope)()
  if !platform.BoolValue(user.IsActive, true) {
		logger.E("User is not active, fallback to denied")
		return false
	}

	scope = strings.ToLower(strings.TrimSpace(scope))
	for _, right := range user.Rights {
		if strings.ToLower(strings.TrimSpace(right)) == scope {
			return true
		}
	}

	logger.W("User has no right", tracing.Scope, scope)
	return false
}

func (x *RightsRepository) AddRightForUser(logger *tracing.Logger, user *entities.User, scope string) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Rights add right for user completed", "repository.rights.add.right.for.user", "user_id", user.ID, "scope", scope)()
	if !slices.Contains(AvailableRights, scope) {
		logger.E("Right not found", tracing.Scope, scope)
		return nil, ErrRightNotFound
	}

	if slices.Contains(user.Rights, scope) {
		logger.E("Right already exists", tracing.Scope, scope)
		return nil, ErrNothingChanged
	}

	user.Rights = append(user.Rights, scope)
	return x.users.UpdateUser(logger, user)
}

func (x *RightsRepository) RemoveRightForUser(logger *tracing.Logger, user *entities.User, scope string) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Rights remove right for user completed", "repository.rights.remove.right.for.user", "user_id", user.ID, "scope", scope)()
	if !slices.Contains(AvailableRights, scope) {
		logger.E("Right not found", tracing.Scope, scope)
		return nil, ErrRightNotFound
	}

	if !slices.Contains(user.Rights, scope) {
		logger.E("Right can't be removed, because it's not present", tracing.Scope, scope)
		return nil, ErrNothingChanged
	}

	user.Rights = slices.DeleteFunc(user.Rights, func(right string) bool {
		return strings.ToLower(strings.TrimSpace(right)) == scope
	})

	return x.users.UpdateUser(logger, user)
}