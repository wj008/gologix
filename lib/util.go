package lib

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//RandInt64 获取随机数
func RandInt64(max int64) int64 {
	if max <= 0 {
		return 0
	}
	rand.Seed(time.Now().UnixNano())
	return rand.Int63n(max)
}

//ParseTagName 拆分数组标签
func ParseTagName(tagName string) (string, []int) {
	re := regexp.MustCompile(`(?i)^([\w.-]+)\[(\d+(:?,\d+)*)\]$`)
	temp := re.FindStringSubmatch(tagName)
	if temp == nil {
		re2 := regexp.MustCompile(`(?i)^([\w.-]+)\.(\d+)$`)
		temp = re2.FindStringSubmatch(tagName)
	}
	if temp == nil {
		return tagName, []int{0}
	}
	numbs := make([]int, 0)
	numArr := strings.Split(temp[2], ",")
	for _, num := range numArr {
		index, _ := strconv.Atoi(num)
		numbs = append(numbs, index)
	}
	if len(numbs) == 0 {
		return tagName, []int{0}
	}
	return temp[1], numbs
}

//IsInteger 是否字符串是数字
func IsInteger(tagName string) bool {
	re := regexp.MustCompile(`(?i)^\d+$`)
	return re.MatchString(tagName)
}

//IsBitWord 是否带有.属性
func IsBitWord(tagName string) bool {
	re := regexp.MustCompile(`(?i)\.\d+$`)
	return re.MatchString(tagName)
}

//GetWordCount 计算字符数量
func GetWordCount(start uint16, length uint16, bits uint16) uint16 {
	newStart := start % bits
	newEnd := newStart + length
	totalWords := (newEnd - 1) / bits
	return totalWords + 1
}
