package proto

import (
	"log"
	"regexp"
	"strconv"
)

var tgFromIdMask = regexp.MustCompile(`tgv\d-fromid-(\d+)`)

func genericMatch(re *regexp.Regexp, s string, index int) int64 {
	match := re.FindStringSubmatch(s)
	if len(match) != index+1 {
		log.Println("Parse", s, "failed with regex groups mismatch")
		return 0
	}

	i, err := strconv.ParseInt(match[index], 10, 64)
	if err != nil {
		log.Println("Parse", s, "failed with strconv error", err)
		return 0
	}

	return i
}

func GetSourceID(m *FLO_SOURCE) int64 {
	return genericMatch(tgFromIdMask, m.SourceUid, 1)
}

func GetMessageID(m *FLO_MESSAGE) int64 {
	return genericMatch(tgFromIdMask, m.MessageUid, 1)

}
