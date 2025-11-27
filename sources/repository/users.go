package repository

import (
	"context"
	"errors"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidUsername = errors.New("invalid username")
)

type UsersRepository struct{}

func NewUsersRepository() *UsersRepository {
	return &UsersRepository{}
}

func (x *UsersRepository) CreateUser(logger *tracing.Logger, euid int64, uname *string, ufullname *string) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Users create user completed", "repository.users.create.user", "user_id", euid)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	user := &entities.User{
		UserID:   euid,
		Username: uname,
		Fullname: ufullname,
		IsActive: platform.BoolPtr(true),
	}

	err := q.User.Create(user)
	if err != nil {
		logger.E("Failed to create user", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created user")
	return user, nil
}

func (x *UsersRepository) MustCreateUser(logger *tracing.Logger, euid int64, uname *string, ufullname *string) *entities.User {
	user, err := x.CreateUser(logger, euid, uname, ufullname)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}

	return user
}

func (x *UsersRepository) GetUserByEid(logger *tracing.Logger, euid int64) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Users get user by eid completed", "repository.users.get.user.by.eid", "user_id", euid)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	user, err := q.User.Where(query.User.UserID.Eq(euid)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.W("User not found when expected")
			return nil, ErrUserNotFound
		} else {
			logger.E("Failed to get user", tracing.InnerError, err)
			return nil, err
		}
	}

	logger.I("User fetched")
	return user, nil
}

func (x *UsersRepository) MustGetUserByEid(logger *tracing.Logger, euid int64) *entities.User {
	user, err := x.GetUserByEid(logger, euid)
	if err != nil {
		logger.F("Got error while expecting user", tracing.InnerError, err)
	}

	return user
}

func (x *UsersRepository) GetUserByName(logger *tracing.Logger, uname string) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Users get user by name completed", "repository.users.get.user.by.name", "username", uname)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	uname = strings.TrimSpace(strings.TrimPrefix(uname, "@"))

	if uname == "" {
		return nil, ErrInvalidUsername
	}

	q := query.Q.WithContext(ctx)

	user, err := q.User.Where(query.User.Username.Eq(uname)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.W("User not found when expected")
			return nil, ErrUserNotFound
		} else {
			logger.E("Failed to get user", tracing.InnerError, err)
			return nil, err
		}
	}

	logger.I("User fetched")
	return user, nil
}

func (x *UsersRepository) MustGetUserByName(logger *tracing.Logger, uname string) *entities.User {
	user, err := x.GetUserByName(logger, uname)
	if err != nil {
		logger.F("Got error while expecting user", tracing.InnerError, err)
	}

	return user
}

func (x *UsersRepository) UpdateUser(logger *tracing.Logger, user *entities.User) (*entities.User, error) {
	defer tracing.ProfilePoint(logger, "Users update user completed", "repository.users.update.user", "user_id", user.UserID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.User.Where(query.User.UserID.Eq(user.UserID)).Updates(user)
	if err != nil {
		logger.E("Failed to update user", tracing.InnerError, err)
		return nil, err
	}

	logger.I("User updated")
	return user, nil
}

func (x *UsersRepository) MustUpdateUser(logger *tracing.Logger, user *entities.User) *entities.User {
	user, err := x.UpdateUser(logger, user)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}

	return user
}

func (x *UsersRepository) DeleteUserByEid(logger *tracing.Logger, euid int64) error {
	defer tracing.ProfilePoint(logger, "Users delete user by eid completed", "repository.users.delete.user.by.eid", "user_id", euid)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.User.Where(query.User.UserID.Eq(euid)).Delete(&entities.User{})
	if err != nil {
		logger.E("Failed to delete user", tracing.InnerError, err)
		return err
	}

	logger.I("User deleted")
	return nil
}

func (x *UsersRepository) MustDeleteUserByEid(logger *tracing.Logger, euid int64) {
	err := x.DeleteUserByEid(logger, euid)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}
}

func (x *UsersRepository) DeleteUserByName(logger *tracing.Logger, uname string) error {
	defer tracing.ProfilePoint(logger, "Users delete user by name completed", "repository.users.delete.user.by.name", "username", uname)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.User.Where(query.User.Username.Eq(uname)).Delete(&entities.User{})
	if err != nil {
		logger.E("Failed to delete user", tracing.InnerError, err)
		return err
	}

	logger.I("User deleted")
	return nil
}

func (x *UsersRepository) MustDeleteUserByName(logger *tracing.Logger, uname string) {
	err := x.DeleteUserByName(logger, uname)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}
}

func (x *UsersRepository) DeleteUser(logger *tracing.Logger, user *entities.User) error {
	defer tracing.ProfilePoint(logger, "Users delete user completed", "repository.users.delete.user", "user_id", user.UserID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.User.Where(query.User.UserID.Eq(user.UserID)).Delete(&entities.User{})
	if err != nil {
		logger.E("Failed to delete user", tracing.InnerError, err)
		return err
	}

	logger.I("User deleted")
	return nil
}

func (x *UsersRepository) MustDeleteUser(logger *tracing.Logger, user *entities.User) {
	err := x.DeleteUser(logger, user)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}
}

func (x *UsersRepository) GetTotalUsersCount(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Users get total users count completed", "repository.users.get.total.users.count")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	count, err := q.User.Count()
	if err != nil {
		logger.E("Failed to count total users", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}