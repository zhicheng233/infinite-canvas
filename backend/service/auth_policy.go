package service

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var (
	usernamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	passwordLetterPattern = regexp.MustCompile(`[A-Za-z]`)
	passwordDigitPattern  = regexp.MustCompile(`[0-9]`)
)

func normalizeUsername(username string) (string, error) {
	normalized := strings.TrimSpace(username)
	if normalized == "" {
		return "", errors.New("请输入用户名")
	}
	if !usernamePattern.MatchString(normalized) {
		return "", errors.New("用户名仅支持1-64位字母、数字、下划线或短横线")
	}
	return normalized, nil
}

func validatePasswordStrength(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return errors.New("密码至少需要8个字符")
	}
	hasLetter := passwordLetterPattern.MatchString(password)
	hasDigit := passwordDigitPattern.MatchString(password)
	if !hasLetter || !hasDigit {
		return errors.New("密码需包含字母和数字")
	}
	return nil
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
