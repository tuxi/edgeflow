package apns

import (
	"crypto/tls"
	"crypto/x509"
	"edgeflow/conf"
	"edgeflow/pkg/logger"
	"fmt"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"golang.org/x/net/http2"
	"net/http"
	"strings"
	"time"
)

type PushMessage struct {
	//DeviceToken string `form:"-" json:"-" xml:"-" query:"-"`
	Category string `form:"category,omitempty" json:"category,omitempty" xml:"category,omitempty" query:"category,omitempty"`
	Title    string `form:"title,omitempty" json:"title,omitempty" xml:"title,omitempty" query:"title,omitempty"`
	Body     string `form:"body,omitempty" json:"body,omitempty" xml:"body,omitempty" query:"body,omitempty"`
	// ios notification sound(system sound please refer to http://iphonedevwiki.net/index.php/AudioServices)
	Sound     string                 `form:"sound,omitempty" json:"sound,omitempty" xml:"sound,omitempty" query:"sound,omitempty"`
	ExtParams map[string]interface{} `form:"ext_params,omitempty" json:"ext_params,omitempty" xml:"ext_params,omitempty" query:"ext_params,omitempty"`
}

type PushResponse struct {
	ApnsID string
	Reason string
}

// 鉴权方式：1.基于token的推送 2.基于p12证书的推送
type Apns struct {
	cfg    *conf.Apns
	client *apns2.Client
}

// 根据证书创建APNS
func NewTokenApns() *Apns {
	cfg := &conf.AppConfig.Apple.Apns
	if cfg == nil {
		logger.Fatalf("Apns is not config")
	}
	// 基于token的方式：apnsPrivateKey 是在apple dev官网 - 用户与访问权限中创建的
	authKey, err := token.AuthKeyFromBytes([]byte(apnsPrivateKey))
	if err != nil {
		logger.Fatalf("failed to create APNS auth key: %v", err)
	}

	var rootCAs *x509.CertPool
	rootCAs, err = x509.SystemCertPool()
	if err != nil {
		logger.Fatalf("failed to get rootCAs: %v", err)
	}

	for _, ca := range apnsCAs {
		rootCAs.AppendCertsFromPEM([]byte(ca))
	}

	return &Apns{
		cfg,
		&apns2.Client{
			Token: &token.Token{
				AuthKey: authKey,
				KeyID:   cfg.KeyID,
				TeamID:  cfg.TeamID,
			},
			HTTPClient: &http.Client{
				Transport: &http2.Transport{
					DialTLS: apns2.DialTLS,
					TLSClientConfig: &tls.Config{
						RootCAs: rootCAs,
					},
				},
				Timeout: apns2.HTTPClientTimeout,
			},
			Host: apns2.HostDevelopment,
		},
	}
}

// 根据证书创建APNS
func NewApns() *Apns {
	cfg := &conf.AppConfig.Apple.Apns
	if cfg == nil {
		logger.Fatalf("Apns is not config")
	}
	// 基于token的方式：
	cert, err := certificate.FromP12File("deploy/push_dev.p12", "")
	if err != nil {
		logger.Fatalf("Cert Error: %v", err.Error())
	}

	return &Apns{
		cfg,
		apns2.NewClient(cert).Development(),
	}
}

func (a *Apns) Push(msg *PushMessage, deviceToken string) (res *PushResponse, err error) {
	if msg == nil {
		return nil, fmt.Errorf("APNS push failed :%s", "无效的message")
	}
	pl := payload.NewPayload().AlertTitle(msg.Title).AlertBody(msg.Body).Sound(msg.Sound).Category(msg.Category)
	group, exist := msg.ExtParams["group"]
	if exist {
		pl = pl.ThreadID(group.(string))
	}

	for k, v := range msg.ExtParams {
		pl.Custom(strings.ToLower(k), fmt.Sprintf("%v", v))
	}

	resp, err := a.client.Push(&apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.cfg.Topic,
		Expiration:  time.Now().Add(24 * time.Hour),
		Payload:     pl.MutableContent(),
	})

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("APNS push failed :%s", resp.Reason)
	}
	res = &PushResponse{
		ApnsID: resp.ApnsID,
		Reason: resp.Reason,
	}
	return
}
