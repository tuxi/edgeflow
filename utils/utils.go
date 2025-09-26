package utils

import (
	"context"
	"edgeflow/internal/consts"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"
)

// RandString generate rand string with specified length
func RandString(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	data := []byte(str)
	var result []byte
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		result = append(result, data[r.Intn(len(data))])
	}
	return string(result)
}

func Long2IP(ipInt int64) string {
	b0 := strconv.FormatInt((ipInt>>24)&0xff, 10)
	b1 := strconv.FormatInt((ipInt>>16)&0xff, 10)
	b2 := strconv.FormatInt((ipInt>>8)&0xff, 10)
	b3 := strconv.FormatInt(ipInt&0xff, 10)
	return b0 + "." + b1 + "." + b2 + "." + b3
}

func ContainsStr(slice []string, item string) bool {
	for _, e := range slice {
		if e == item {
			return true
		}
	}
	return false
}

// Stamp2str 时间戳转字符串
func Stamp2str(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format(consts.TimeLayout)
}

// Str2stamp 字符串转时间戳
func Str2stamp(str string) int64 {
	t, err := time.Parse(consts.TimeLayout, str)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// 获取有效的utf8编码字符串
func ValidUTF8String(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	v := make([]rune, 0, len(s))
	for i, r := range s {
		if r == utf8.RuneError {
			_, size := utf8.DecodeRuneInString(s[i:])
			if size == 1 {
				continue
			}
		}
		v = append(v, r)
	}
	return string(v)
}

func DownloadImageWithURL(ctx context.Context, url string, callback func(receivedSize int64, totalSize int64)) ([]byte, error) {
	cl := http.Client{Timeout: time.Second * 60}
	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cl.Do(downloadReq)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Download image failure")
	}

	// Get the total size from the Content-Length header
	totalSize, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	// Create our progress reporter and pass it to be used alongside our writer
	counter := &ProgressWriter{
		TotalSize: totalSize,
		callback: func(receivedSize int64, totalSize int64) {
			callback(receivedSize, totalSize)
		},
	}
	// io.Copy read 方法 会执行接口 io.TeeReader （read -> writer 方法）
	respBody := io.TeeReader(resp.Body, counter)
	bytes, err := io.ReadAll(respBody)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

type ProgressWriter struct {
	TotalSize    int64
	ReceivedSize int64
	CurrentStep  int
	Response     http.ResponseWriter
	callback     func(receivedSize int64, totalSize int64)
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.ReceivedSize += int64(n)

	pw.callback(pw.ReceivedSize, pw.TotalSize)
	return n, nil
}
