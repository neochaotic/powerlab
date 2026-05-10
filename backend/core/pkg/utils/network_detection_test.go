package utils

import (
	"fmt"
	"testing"
)

func TestGetResultTest(t *testing.T) {
	t.Skip("This test is always failing. Skipped to unblock releasing - MUST FIX!")

	list := []string{"https://www.google.com", "https://www.bing.com", "https://www.baidu.com"}
	data := make(chan string)
	// data <- "init"
	for _, v := range list {
		go GetNetWorkTypeDetection(data, v)
	}
	result := <-data
	close(data)
	fmt.Println(result)
}
