package domain

import "errors"

// Empty values preserve IPv4 behavior for existing configurations and backups.
func ListenFamilies(value string) []string {
	if value == "dual" {
		return []string{"ipv4", "ipv6"}
	}
	if value == "ipv6" {
		return []string{"ipv6"}
	}
	return []string{"ipv4"}
}

func ValidateListenType(value string) error {
	switch value {
	case "", "ipv4", "ipv6", "dual":
		return nil
	default:
		return errors.New("监听类型必须为 IPv4、IPv6 或双栈")
	}
}
