package service

import (
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/utils"
)

var (
	ErrInvalidPhone          = errors.New("invalid phone number")
	ErrPhoneAlreadyExists    = errors.New("phone already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrUserDisabled          = errors.New("user disabled")
)

// ValidateUsername 校验用户名：仅支持英文+数字+下划线，长度3-32
func ValidateUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	return matched
}

type UserService struct {
	userRepo   *repository.UserRepository
	apiKeyRepo *repository.ApiKeyRepository
}

func NewUserService(userRepo *repository.UserRepository, apiKeyRepo *repository.ApiKeyRepository) *UserService {
	return &UserService{
		userRepo:   userRepo,
		apiKeyRepo: apiKeyRepo,
	}
}

// Register 用户注册（手机号+用户名必填；验证码已在 handler 层校验）
// referrerType / referrerID 为该用户的注册归因推介方（空/0 表示平台直营），由调用方按推广码解析
func (s *UserService) Register(phone, username, password, referrerType string, referrerID int64) (*model.User, error) {
	// 校验用户名格式
	if !ValidateUsername(username) {
		return nil, ErrInvalidUsername
	}

	// 检查手机号是否已存在
	exists, err := s.userRepo.CheckPhoneExists(phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPhoneAlreadyExists
	}

	// 检查用户名是否已存在
	exists, err = s.userRepo.CheckUsernameExists(username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameAlreadyExists
	}

	// 密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &model.User{
		Phone:          phone,
		Username:       username,
		PasswordHash:   string(hashedPassword),
		Balance:        0,
		RealnameStatus: model.RealnameNone,
		Status:         1,
		ReferrerType:   referrerType,
		ReferrerID:     referrerID,
	}

	err = s.userRepo.CreateUser(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Login 用户登录（支持用户名/手机号）
func (s *UserService) Login(account, password string) (*model.User, error) {
	// 查询用户
	user, err := s.userRepo.GetUserByAccount(account)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, ErrInvalidPassword
		}
		return nil, err
	}

	// 校验密码
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidPassword
	}

	// 检查用户状态
	if user.Status == 0 {
		return nil, ErrUserDisabled
	}

	// 更新最后登录时间
	_ = s.userRepo.UpdateLastLoginTime(user.ID)

	return user, nil
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(id int64) (*model.User, error) {
	return s.userRepo.GetUserByID(id)
}

// GetUserByPhone 根据手机号获取用户
func (s *UserService) GetUserByPhone(phone string) (*model.User, error) {
	return s.userRepo.GetUserByPhone(phone)
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(userID int64, newPassword string) error {
	// 密码哈希
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdateUserPassword(userID, string(passwordHash))
}

// MaxAPIKeysPerUser 单账号可创建的 API 密钥数量上限
const MaxAPIKeysPerUser = 10

// CreateAPIKey 为用户创建一把 API 密钥（名称 + 权限范围，权限为 all 或端点标识集合）
func (s *UserService) CreateAPIKey(userID int64, name, permission string) (*model.ApiKey, error) {
	n, err := s.apiKeyRepo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	if n >= MaxAPIKeysPerUser {
		return nil, fmt.Errorf("最多可创建 %d 把 API 密钥", MaxAPIKeysPerUser)
	}
	k := &model.ApiKey{
		UserID:     userID,
		Name:       name,
		APIKey:     utils.GenerateRandomKey(32),
		APISecret:  utils.GenerateRandomKey(32),
		Permission: permission,
	}
	if err := s.apiKeyRepo.Create(k); err != nil {
		return nil, err
	}
	return k, nil
}

// ListAPIKeys 查询用户全部 API 密钥（按创建顺序，首把为该账号主密钥）
func (s *UserService) ListAPIKeys(userID int64) ([]*model.ApiKey, error) {
	return s.apiKeyRepo.ListByUser(userID)
}

// UpdateAPIKey 修改指定密钥的名称与权限范围（限本人密钥）
func (s *UserService) UpdateAPIKey(userID, id int64, name, permission string) error {
	return s.apiKeyRepo.UpdateKey(id, userID, name, permission)
}

// DeleteAPIKey 删除指定密钥（限本人密钥）
func (s *UserService) DeleteAPIKey(userID, id int64) error {
	return s.apiKeyRepo.DeleteByID(id, userID)
}

// GetAPIKeyByUser 查询用户主密钥（最早创建的一把）
func (s *UserService) GetAPIKeyByUser(userID int64) (*model.ApiKey, error) {
	return s.apiKeyRepo.GetByUser(userID)
}
