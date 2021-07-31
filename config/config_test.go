package config

import (
	"fmt"
	"testing"
)

func TestConfigTest(t *testing.T) {
	var err error
	constConf := MyConst{}
	err = readConst(&constConf, "./const.yml")
	if err != nil {
		panic(err)
	}
	logger := MyConst{}
	err = readConst(&logger, "./logger.yml")
	if err != nil {
		panic(err)
	}
	constConf.Logger = logger.Logger
	fmt.Printf("%+v\n", constConf.MyAuth)
	fmt.Printf("%+v\n", constConf.Cluster)
	fmt.Printf("%+v\n", constConf.DefaultConst)
	fmt.Printf("%+v\n", constConf.MyBuff)
	fmt.Printf("%+v\n", constConf.ServerVersion)
	fmt.Printf("%+v\n", constConf.Logger)
}
