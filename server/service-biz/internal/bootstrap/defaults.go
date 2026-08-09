package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type counterSeed struct {
	Name  string
	Value int64
}

func defaultOperator(now int64) (model.Operator, error) {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("SLAN_OPS_DEFAULT_ADMIN_EMAIL")))
	if email == "" {
		email = "admin1"
	}
	password := strings.TrimSpace(os.Getenv("SLAN_OPS_DEFAULT_ADMIN_PASSWORD"))
	if password == "" {
		password = "admin1"
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.Operator{}, fmt.Errorf("hash bootstrap admin password: %w", err)
	}
	return model.Operator{
		OperatorID:   "op00000000000000000000000000000001",
		Email:        email,
		Name:         "超级管理员",
		PasswordHash: string(passwordHash),
		Role:         "admin",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func defaultSeedCounters() []counterSeed {
	return []counterSeed{
		{Name: "operator", Value: 1},
	}
}
