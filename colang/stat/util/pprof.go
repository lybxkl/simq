package util

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
)

func Profiling(port int) {
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}()
}
