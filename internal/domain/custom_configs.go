package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var customConfigNamePattern = regexp.MustCompile(`^custom-[A-Za-z0-9][A-Za-z0-9._-]{0,71}\.(conf|stream)$`)

func ValidateCustomConfigs(configs []CustomConfig) error {
	if len(configs) > 100 {
		return errors.New("自定义配置文件最多支持 100 个")
	}
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		if !customConfigNamePattern.MatchString(config.Name) {
			return fmt.Errorf("自定义配置文件名 %q 不合法，必须以 custom- 开头并以 .conf 或 .stream 结尾", config.Name)
		}
		key := strings.ToLower(config.Name)
		if _, exists := seen[key]; exists {
			return errors.New("存在重复的自定义配置文件名")
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(config.Content) == "" {
			return fmt.Errorf("自定义配置文件 %q 不能为空", config.Name)
		}
		if len(config.Content) > 1024*1024 {
			return fmt.Errorf("自定义配置文件 %q 不能超过 1 MB", config.Name)
		}
	}
	return nil
}
