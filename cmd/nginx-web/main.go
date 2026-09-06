package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/chenpingonline/fn-nginx-web/internal/acme"
	"github.com/chenpingonline/fn-nginx-web/internal/app"
	"github.com/chenpingonline/fn-nginx-web/internal/domain"
	"github.com/chenpingonline/fn-nginx-web/internal/platform"
	"github.com/chenpingonline/fn-nginx-web/internal/service"
	webassets "github.com/chenpingonline/fn-nginx-web/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "acme-worker" {
		os.Exit(acme.RunWorker())
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	paths, err := platform.LoadPaths()
	fatalIf(err)
	appService, err := service.New(paths)
	fatalIf(err)
	application := app.New(paths, appService, webassets.Assets)

	switch command {
	case "serve":
		fatalIf(application.Serve())
	case "init":
		result, err := appService.Prepare()
		fatalIf(err)
		printJSON(result)
	case "nginx-start":
		result, err := appService.NginxStart()
		fatalIf(err)
		printJSON(result)
	case "nginx-stop":
		result, err := appService.NginxStop()
		fatalIf(err)
		printJSON(result)
	case "nginx-reload":
		result, err := appService.NginxReload()
		fatalIf(err)
		printJSON(result)
	case "nginx-test":
		result, err := appService.NginxTest()
		fatalIf(err)
		printJSON(result)
	case "doctor":
		printJSON(application.Doctor())
	case "version", "--version", "-v":
		fmt.Printf("%s (Nginx %s)\n", domain.BuildIdentity, domain.NginxVersion)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", command)
		fmt.Fprintln(os.Stderr, "可用命令: serve, init, nginx-start, nginx-stop, nginx-reload, nginx-test, doctor, version")
		os.Exit(2)
	}
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func fatalIf(err error) {
	if err == nil {
		return
	}
	log.Print(err)
	os.Exit(1)
}
