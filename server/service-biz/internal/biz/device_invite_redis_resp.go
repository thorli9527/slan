package biz

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type redisResp struct {
	simple string
	bulk   string
	nil    bool
}

func redisCommand(args ...string) []byte {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, arg := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteString("\r\n")
		b.WriteString(arg)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func readRedisResp(reader *bufio.Reader) (redisResp, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return redisResp{}, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return redisResp{}, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return redisResp{simple: line}, nil
	case '-':
		return redisResp{}, errors.New(line)
	case ':':
		return redisResp{simple: line}, nil
	case '$':
		size, err := strconv.Atoi(line)
		if err != nil {
			return redisResp{}, err
		}
		if size < 0 {
			return redisResp{nil: true}, nil
		}
		buf := make([]byte, size+2)
		if _, err := reader.Read(buf); err != nil {
			return redisResp{}, err
		}
		return redisResp{bulk: string(buf[:size])}, nil
	default:
		return redisResp{}, fmt.Errorf("unsupported redis response %q", prefix)
	}
}
