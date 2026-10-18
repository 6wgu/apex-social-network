package mail

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host     string
	Port     int
	Secure   bool
	User     string
	Pass     string
	FromName string

	OTPLength        int
	OTPTTLMinutes    int
	OTPMaxAttempts   int
	OTPResendSeconds int
}

func LoadDotEnv(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		path := filepath.Join(d, ".env")
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				k, v, ok := strings.Cut(line, "=")
				if !ok {
					continue
				}
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && strings.IndexByte(v[1:], v[0]) >= 0 {
					v = v[1 : 1+strings.IndexByte(v[1:], v[0])]
				} else if i := strings.Index(v, " #"); i >= 0 {
					v = strings.TrimSpace(v[:i])
				} else if strings.HasPrefix(v, "#") {
					v = ""
				}
				if _, set := os.LookupEnv(k); !set {
					os.Setenv(k, v)
				}
			}
			return path
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

func FromEnv() Config {
	num := func(name string, fallback int) int {
		if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
			return n
		}
		return fallback
	}
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "smtp.gmail.com"
	}
	from := os.Getenv("MAIL_FROM_NAME")
	if from == "" {
		from = "Social Network"
	}
	return Config{
		Host:     host,
		Port:     num("SMTP_PORT", 587),
		Secure:   os.Getenv("SMTP_SECURE") == "true",
		User:     os.Getenv("SMTP_USER"),
		Pass:     strings.Join(strings.Fields(os.Getenv("SMTP_PASS")), ""),
		FromName: from,

		OTPLength:        num("OTP_LENGTH", 6),
		OTPTTLMinutes:    num("OTP_TTL_MINUTES", 10),
		OTPMaxAttempts:   num("OTP_MAX_ATTEMPTS", 5),
		OTPResendSeconds: num("OTP_RESEND_SECONDS", 60),
	}
}
