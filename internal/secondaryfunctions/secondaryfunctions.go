package secondaryfunctions

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
)

func CalculateLuhn(number int) int {
	checkNumber := checksum(number)
	if checkNumber == 0 {
		return 0
	}
	return 10 - checkNumber
}
func checksum(number int) int {
	var luhn int
	for i := 0; number > 0; i++ {
		cur := number % 10
		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}
		luhn += cur
		number = number / 10
	}
	return luhn % 10
}
func GenerateRandomString(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return base32.StdEncoding.EncodeToString(b)
}
func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
