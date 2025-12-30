package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func GetEnv(key, defValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return defValue
	}
	return value
}

func SplitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func IntEnv(key string, defValue int) (int, error) {
	value := GetEnv(key, strconv.Itoa(defValue))
	return strconv.Atoi(value)
}

func Int64Env(key string, defValue int64) (int64, error) {
	value := GetEnv(key, strconv.FormatInt(defValue, 10))
	return strconv.ParseInt(value, 10, 64)
}

func FloatEnv(key string, defValue float64) (float64, error) {
	value := GetEnv(key, strconv.FormatFloat(defValue, 'f', -1, 64))
	return strconv.ParseFloat(value, 64)
}

func DurationEnv(key string, defValue time.Duration) (time.Duration, error) {
	value := GetEnv(key, defValue.String())
	return time.ParseDuration(value)
}
