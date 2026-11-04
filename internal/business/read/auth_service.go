package read

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/desi/crawl-conversations/internal/entity"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Claims struct {
	UserID int64
	Email  string
	Role   string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (string, entity.User, error)
	Verify(token string) (Claims, error)
	SeedAdmin(ctx context.Context, email, password string) error
}

type authService struct {
	users  UserRepo
	secret []byte
	ttl    time.Duration
}

func NewAuthService(users UserRepo, secret string, ttl time.Duration) AuthService {
	return &authService{users: users, secret: []byte(secret), ttl: ttl}
}

func (s *authService) Login(ctx context.Context, email, password string) (string, entity.User, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", entity.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", entity.User{}, ErrInvalidCredentials
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"role":  u.Role,
		"iat":   now.Unix(),
		"exp":   now.Add(s.ttl).Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", entity.User{}, err
	}
	return tok, u, nil
}

func (s *authService) Verify(token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	uid, _ := mc["sub"].(float64)
	email, _ := mc["email"].(string)
	role, _ := mc["role"].(string)
	return Claims{UserID: int64(uid), Email: email, Role: role}, nil
}

func (s *authService) SeedAdmin(ctx context.Context, email, password string) error {
	if password == "" {
		return nil // nothing to seed
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.CreateIfNotExists(ctx, entity.User{
		Email: email, PasswordHash: string(h), Name: "Admin", Role: "admin",
	})
}
