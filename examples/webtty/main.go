package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/poplknight/cosmos/webtty"
)

// 部署到客户服务时，把盐换成自己定义的一串字符，并让客户服务器与本示例用同一个盐。
const secret = "demo-salt-change-me"

func main() {
	period := flag.Duration("period", 10*time.Minute, "动态密码有效期")
	printCode := flag.Bool("code", false, "打印当前动态密码后退出（开发现场用）")
	flag.Parse()

	if *printCode {
		code, err := webtty.CurrentPassword(secret, *period)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(code)
		return
	}

	srv := webtty.New(webtty.WithLocalPassword(secret, *period))
	server := http.Server{
		Addr:    ":8787",
		Handler: srv.Mount(""),
	}
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
