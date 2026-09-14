// Package version отдаёт версию приложения
// Источник правды в git: файл VERSION в корне репозитория
package version

import (
	"os"
	"strings"
	"sync"
)

// Version можно переопределить при сборке через ldflags
// go build -ldflags "-X github.com/hovergo/rqbin/internal/version.Version=0.1.0"
var Version string

var (
	once     sync.Once
	resolved string
)

// Current возвращает версию из ldflags или из файла VERSION
func Current() string {
	once.Do(func() {
		if strings.TrimSpace(Version) != "" {
			resolved = strings.TrimSpace(Version)
			return
		}

		data, err := os.ReadFile("VERSION")
		if err != nil {
			resolved = "dev"
			return
		}
		resolved = strings.TrimSpace(string(data))
		if resolved == "" {
			resolved = "dev"
		}
	})

	return resolved
}
