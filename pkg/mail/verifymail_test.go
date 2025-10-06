/*
 * @Author: cloudyi.li
 * @Date: 2023-05-31 17:32:37
 * @LastEditTime: 2023-05-31 23:09:28
 * @LastEditors: cloudyi.li
 * @FilePath: /chatserver-api/pkg/mail/verifymail_test.go
 */
package mail

import (
	"fmt"
	"testing"
)

func Test_verifierEmail(t *testing.T) {
	//c := config.Load("../../config.test/config.toml")
	//logger.InitLogger(&c.LogConfig, c.AppName)

	type args struct {
		email string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test1",
			args: args{email: "44553@gmail.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := NewVerifier()
			err := ver.VerifierEmail(tt.args.email)
			if err != nil {
				fmt.Println(err)
			}
		})
	}
}
