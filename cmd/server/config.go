package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	Addr      string
	Database  string
	SelfCheck bool
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	cfg := config{}
	fs.StringVar(&cfg.Addr, "addr", "127.0.0.1:19081", "HTTP 监听地址")
	fs.StringVar(&cfg.Database, "db", "quarantine.db", "SQLite 数据库文件")
	fs.BoolVar(&cfg.SelfCheck, "self-check", false, "运行完整 HTTP 自检后退出")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	addrSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})
	if !addrSet {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return cfg, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			cfg.Addr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil || port == "" {
		return cfg, fmt.Errorf("-addr 必须为 host:port")
	}
	if host == "" {
		return cfg, fmt.Errorf("监听地址必须明确指定主机")
	}
	if cfg.SelfCheck {
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return cfg, fmt.Errorf("自检仅允许绑定回环地址")
		}
	}
	return cfg, nil
}
