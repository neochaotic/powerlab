package utils

import natType "github.com/Curtis-Milo/nat-type-identifier-go"

/**
 * @description:
 * @param {chanstring} data
 * @param {string} url
 * @return {*}
 */
func GetNetWorkTypeDetection(data chan string, url string) {
	// fmt.Println("url:", url)
	// httper.Get(url, nil)
	// aaa <- url
	result, err := natType.GetDeterminedNatType(true, 5, url)
	if err == nil {
		data <- result
	}

}
