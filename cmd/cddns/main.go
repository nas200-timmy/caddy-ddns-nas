// cddns - 域名代理 + 免费证书一站式部署工具
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cddns/internal/config"
	"cddns/internal/service"
	"cddns/internal/tui"
)

// version 由 Makefile 通过 -ldflags 注入
var version = "dev"

func main() {
	useDefaultDataDir()

	if len(os.Args) < 2 {
		rerun, err := tui.RunPanel(config.DefaultPath())
		if err != nil {
			fmt.Fprintln(os.Stderr, "cddns:", err)
			os.Exit(1)
		}
		if rerun {
			ensureDataDir()
			if err := runSetup(nil); err != nil {
				fmt.Fprintln(os.Stderr, "cddns setup:", err)
				os.Exit(1)
			}
		}
		return
	}
	switch os.Args[1] {
	case "setup":
		ensureDataDir()
		if err := runSetup(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "cddns setup:", err)
			os.Exit(1)
		}
	case "run":
		ensureDataDir()
		if err := runService(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "cddns run:", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println("cddns", version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(1)
	}
}

// useDefaultDataDir 统一证书存储位置：向导、面板与服务共用同一份 XDG_DATA_HOME。
func useDefaultDataDir() {
	if os.Getenv("XDG_DATA_HOME") == "" {
		os.Setenv("XDG_DATA_HOME", config.DefaultDataDir())
	}
}

// ensureDataDir 在确实要写入（向导签发证书 / 服务运行）时创建数据目录。
func ensureDataDir() {
	dir := os.Getenv("XDG_DATA_HOME")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cddns: 数据目录 %s 不可用: %v（可用 CDDNS_DATA_DIR 指定其他目录）\n", dir, err)
	}
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return tui.RunWizard(*cfgPath)
}

func runService(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return service.Run(ctx, *cfgPath)
}

func usage() {
	fmt.Println(`cddns - 域名代理 + 免费证书一站式部署工具

用法:
  cddns setup [--config 路径]   部署向导（一步步配置源站、DNS 凭证、域名）
  cddns run [--config 路径]     headless 运行服务（代理 + DDNS 看门狗）
  cddns version                 显示版本

环境变量:
  CDDNS_CONFIG     配置文件路径（默认 /etc/cddns/config.yaml）
  CDDNS_DATA_DIR   数据与证书目录（默认 /var/lib/cddns）`)
}
