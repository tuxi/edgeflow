package admin

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/pkg/logger"
	"edgeflow/pkg/push/apns"
	"math/rand"
	"time"
)

var (
	cli           *apns.Apns
	WaitPushChan  chan PushMessageReq // 等待发送的通道
	responses     chan apns.PushResponse
	WaitQueryChan chan apns.PushMessage // 等待查询的通道
	deviceDao     dao.DeviceDao
)

type PushMessageReq struct {
	*apns.PushMessage
	DeviceToken string
}

func InitApnsServer(dd dao.DeviceDao) {
	logger.Info("Start push main loop")
	deviceDao = dd
	WaitPushChan = make(chan PushMessageReq, 3000)
	responses = make(chan apns.PushResponse, 3000)
	WaitQueryChan = make(chan apns.PushMessage, 3000)
	cli = apns.NewTokenApns()
	for i := 0; i < 10; i++ {
		go workerSend()
	}
	for i := 0; i < 10; i++ {
		go workResponse()
	}
	startLoop()
}

func Push(m *apns.PushMessage, deviceToken string) error {
	res, err := cli.Push(m, deviceToken)
	logger.Debugf("ApnsId: %v, Reason: %v", res.ApnsID, res.Reason)
	return err
}

func PushAllDevice(m *apns.PushMessage) {
	if m == nil {
		return
	}
	WaitQueryChan <- *m
}

// 创建worker协程用于接受NotiChan中发来的notification，并发送到用户
func workerSend() {
	for {
		n := <-WaitPushChan
		logger.Debugf("Start sending notification")
		res, err := cli.Push(n.PushMessage, n.DeviceToken)
		if err != nil {
			logger.Fatalf("Push error: %v", err)
		}
		responses <- *res
	}
}

// 创建协程，处理response
func workResponse() {
	for {
		res := <-responses
		logger.Debugf("%v", res.ApnsID)
	}
}

func startLoop() {
	counter := 0
	go func() {
		for {

			waitQueryMessage := <-WaitQueryChan

			tokens, err := deviceDao.UserDeviceTokenGetList(context.Background())
			if err != nil {
				logger.Fatalf(err.Error())
			}

			var message = PushMessageReq{
				PushMessage: &waitQueryMessage,
				DeviceToken: "",
			}
			for _, item := range tokens {
				message.DeviceToken = item.DeviceToken
				WaitPushChan <- message
			}

			waitTime := rand.Intn(10)
			logger.Infof("wait: %v seconds", waitTime)
			time.Sleep(time.Duration(waitTime) * time.Second)
			counter = counter + 1
		}
	}()
}
