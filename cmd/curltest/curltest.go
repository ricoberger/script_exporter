package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(1)
	}

	t := os.Args[1]
	res, err := http.Get(t)
	if err != nil {
		os.Exit(1)
	}

	defer res.Body.Close()

	fmt.Printf("# HELP curl_status_code returns the status code of the target\n")
	fmt.Printf("# TYPE curl_status_code gauge\n")
	fmt.Printf("curl_status_code{target=\"%s\"} %d\n", t, res.StatusCode)
}
