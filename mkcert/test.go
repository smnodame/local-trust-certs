package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func x(cmd ...string) {
	// service := "proxy"
	caddyfile_path := "../Caddyfile"
	new_tls_config := "tls /etc/certs/localhost.crt /etc/certs/localhost.key"

	caddyfile_content, err := os.Open(caddyfile_path)
	if err != nil {
		log.Fatal("can not find caddyfile from path that you have defined")
	}

	var lines []string
	scanner := bufio.NewScanner(caddyfile_content)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var new_lines []string
	has_tls := false
	for _, line := range lines {
		if strings.Contains(line, "{") {
			has_tls = false
		}

		if strings.Contains(line, "tls") {
			has_tls = true
			new_lines = append(new_lines, new_tls_config)
			continue
		}

		if strings.Contains(line, "}") {
			if has_tls == false {
				new_lines = append(new_lines, new_tls_config)
			}
		}

		new_lines = append(new_lines, line)
	}

	log.Println(new_lines)
}
