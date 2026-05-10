package encryption

import (
	"crypto/md5"
	"encoding/hex"
)

func GetMD5ByStr(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
