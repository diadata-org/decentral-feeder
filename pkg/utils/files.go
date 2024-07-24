package utils

import (
	"os"
	"os/user"
)

func GetPath(configPath string, exchange string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if dir == "/root" || dir == "/home" {
		return "/config/" + configPath + exchange + ".json"
	}
	return os.Getenv("GOPATH") + "/home/shpookas/dia/git/decentralized-feeder/decentral-feeder/config/" + configPath + exchange + ".json"
}
