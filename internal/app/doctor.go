package app

import (
	"os"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

func (a *App) Doctor() map[string]any {
	checks := map[string]any{
		"app_name":      domain.AppName,
		"app_version":   domain.AppVersion,
		"nginx_version": domain.NginxVersion,
		"paths":         a.paths,
		"overview":      a.service.Overview(),
	}
	if err := a.service.CheckNginxBinary(); err != nil {
		checks["binary_error"] = err.Error()
	} else {
		checks["binary_ok"] = true
	}
	if _, err := os.Stat(a.paths.NginxMaster); err == nil {
		result, testErr := a.service.NginxTest()
		if testErr != nil {
			checks["config_error"] = testErr.Error()
		} else {
			checks["config_test"] = result
		}
	}
	return checks
}
