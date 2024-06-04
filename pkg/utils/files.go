package utils

import (
	"os"
	"os/user"
)

func GetPath(configPath string, exchange string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if dir == "/root" || dir == "/home" {
		return "/config/" + exchange + ".json" //hack for docker...
	}
	return os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/" + configPath + exchange + ".json"
}
