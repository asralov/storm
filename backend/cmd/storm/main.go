package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/asralov/storm/internal/app"
	"github.com/asralov/storm/internal/buildinfo"
	"github.com/asralov/storm/internal/webui"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

func main() {
	port := flag.Int("port", 8080, "loopback port (0 selects an available port)")
	noBrowser := flag.Bool("no-browser", false, "do not open dashboard in browser")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("Storm", buildinfo.Version())
		return
	}
	if flag.NArg() != 0 {
		log.Fatal("unexpected positional arguments")
	}
	assets, err := webui.Assets()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = app.Run(ctx, app.Options{Port: *port, Wrap: func(api http.Handler, address string) http.Handler { return webui.Handler(assets, api, address) }, Ready: func(url string) {
		if !*noBrowser {
			if err := openBrowser(url); err != nil {
				log.Printf("Open %s manually (browser launch failed: %v)", url, err)
			}
		}
	}})
	if err != nil {
		log.Fatal(err)
	}
}
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
