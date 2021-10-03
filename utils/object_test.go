package utils

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestObjectId(t *testing.T) {
	fmt.Println(Generate())
	fmt.Println(time.Now().UnixNano())
	fmt.Println(time.Now().UnixMicro())
	fmt.Println(time.Now().UnixMilli())
	fmt.Println(time.Now().Unix())
	b1 := []byte("asd,asd,as")
	s := hex.EncodeToString(b1)
	fmt.Println(s)
	b, _ := hex.DecodeString(s)
	fmt.Println(b, string(b))
	fmt.Println(hex.EncodeToString(b))
}
