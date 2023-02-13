package golib

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

var (
	tagMtx   sync.Mutex
	tag      string
	timeChan = make(chan string)
	strChan  = make(chan string)
)

func GetURL(url string) (string, error) {
	time.Sleep(time.Second * 3)
	//nolint:noctx
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	resData, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(resData), nil
}

func Hello() {
	fmt.Println("Hello!")
}

func SetTag(newt string) {
	tagMtx.Lock()
	tag = newt
	tagMtx.Unlock()
}

// produceTime writes into timeChan
func produceTime() {
	for {
		tagMtx.Lock()
		t := tag
		tagMtx.Unlock()
		s := fmt.Sprintf("%s - %s", t,
			time.Now().Format("2006-01-02 15:04:05.000 MST -0700"))
		timeChan <- s
	}
}

// NextTime reads from timeChan (blocks).
func NextTime() string {
	nt := <-timeChan
	return nt
}

func WriteStr(s string) {
	tagMtx.Lock()
	t := tag
	tagMtx.Unlock()
	strChan <- t + " " + s
}

func ReadStr() string {
	return <-strChan
}

type ReadLoopCB interface {
	F(string)
}

func ReadLoop(cb ReadLoopCB) {
	go func() {
		for {
			cb.F(<-strChan)
		}
	}()
}

func init() {
	go produceTime()
}
