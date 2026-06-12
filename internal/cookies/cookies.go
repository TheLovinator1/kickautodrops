package cookies

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"kickautodrops/internal/log"
)

// Load reads a Netscape-format cookies.txt and returns a map of cookie name → value.
// Expired cookies are silently skipped.
func Load(filePath string) (map[string]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer f.Close()

	now := time.Now().Unix()
	cookies := make(map[string]string)

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNo++

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Netscape format (tab-separated):
		// domain  flag  path  secure  expiry  name  value
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			// Some exporters use spaces instead of tabs — try splitting on whitespace
			parts = strings.Fields(line)
			if len(parts) < 7 {
				continue
			}
		}

		expiry, err := strconv.ParseInt(parts[4], 10, 64)
		if err == nil && expiry > 0 && expiry < now {
			continue // skip expired
		}

		name := parts[5]
		value := parts[6]
		cookies[name] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cookie file: %w", err)
	}

	if len(cookies) == 0 {
		log.Printf("  \u26a0 File '%s' is empty or contains no cookies.\n", filePath)
		return cookies, nil
	}

	log.Printf("  [Cookies] \u2713 Loaded %d cookies from: %s\n", len(cookies), filePath)
	return cookies, nil
}
