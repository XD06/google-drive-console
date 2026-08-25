// Convert Cookie Editor JSON exports to Netscape format for yt-dlp.
// Usage: go run ./cmd/cookieconvert/ bilibili.com.json douyin.com.json > data/cookies.txt
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type cookieEntry struct {
	Domain         string  `json:"domain"`
	ExpirationDate float64 `json:"expirationDate"`
	HostOnly       bool    `json:"hostOnly"`
	HTTPOnly       bool    `json:"httpOnly"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	SameSite       string  `json:"sameSite"`
	Secure         bool    `json:"secure"`
	Value          string  `json:"value"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s file1.json [file2.json ...]\n", os.Args[0])
		os.Exit(1)
	}

	fmt.Println("# Netscape HTTP Cookie File")
	fmt.Println("# This is a generated file!  Do not edit.")
	fmt.Println()

	for _, path := range os.Args[1:] {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			continue
		}
		var cookies []cookieEntry
		if err := json.Unmarshal(data, &cookies); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
			continue
		}
		for _, c := range cookies {
			domain := c.Domain
			includeSubdomains := "TRUE"
			if c.HostOnly {
				includeSubdomains = "FALSE"
			}
			// Strip leading dot for Netscape format consistency
			domain = strings.TrimPrefix(domain, ".")
			if !c.HostOnly {
				domain = "." + domain
			}
			secure := "FALSE"
			if c.Secure {
				secure = "TRUE"
			}
			// Expiration as Unix timestamp (integer)
			expires := strconv.FormatInt(int64(c.ExpirationDate), 10)
			// Netscape format: domain | includeSubdomains | path | secure | expiration | name | value
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				domain, includeSubdomains, c.Path, secure, expires, c.Name, c.Value)
		}
		_ = io.EOF
	}
}
