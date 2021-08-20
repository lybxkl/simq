package utils

import (
	"os"
	"strings"
)

var CfgPathENV = "SI_CFG_PATH"

/*
获取程序运行路径
*/
func GetCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
func GetConfigPath(cd, fileName string) string {
	root := os.Getenv(CfgPathENV)
	if root == "" {
		root = cd + "/config/" + fileName
	} else {
		root = root + "/" + fileName
	}
	return root
}
