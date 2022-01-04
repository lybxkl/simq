package config

import (
	"fmt"
	"testing"
)

func TestCfg(t *testing.T) {
	si := &SIConfig{}
	si.ServerVersion = "3.0"
	si.Broker.RetainAvailable = true
	if e := Validate.Struct(si); e != nil {
		panic(Translate(e))
	}
	fmt.Println(si.String())
}
