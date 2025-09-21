package security

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/curve25519"
	"testing"
)

func TestEncryptionAndDecryption(t *testing.T) {
	// 示例：生成随机的salt和sharedInfo
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		t.Fatal(err)
	}

	sharedInfo := make([]byte, 16)
	_, err = rand.Read(sharedInfo)
	if err != nil {
		t.Fatal(err)
	}

	// 示例：生成Curve25519私钥和公钥
	servicePrivateKey := make([]byte, 32)
	_, err = rand.Read(servicePrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	clientPrivateKey := make([]byte, 32)
	_, err = rand.Read(clientPrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	// 客户端公钥
	clientPublicKey, err := curve25519.X25519(clientPrivateKey, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	// 服务端的公钥是发给客户端用的，客户端使用服务端的公钥加解密
	//servicePublicKey, err := curve25519.X25519(servicePrivateKey, curve25519.Basepoint)
	//if err != nil {
	//	t.Fatal(err)
	//}

	chaCha, err := NewChaChaPoly(servicePrivateKey, clientPublicKey, salt, sharedInfo, nil)
	if err != nil {
		t.Fatal(err)
	}

	originalText := "Hello, World!"

	ciphertext, err := chaCha.Encrypt([]byte(originalText))
	if err != nil {
		t.Fatal(err)
	}

	decryptedText, err := chaCha.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal([]byte(originalText), decryptedText) {
		t.Fatalf("Original text and decrypted text do not match. Original: %s, Decrypted: %s", originalText, string(decryptedText))
	}
}

func TestEncryptionAndDecryption1(t *testing.T) {
	// salt每次改变
	//salt := make([]byte, 16)
	//_, err := rand.Read(salt)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	salt := []byte("Talkify-Talkify")

	// auth是固定的
	sharedInfoStr := "5fuYp+O2f0mRxPMNFtryhw=="
	sharedInfo, err := base64.StdEncoding.DecodeString(sharedInfoStr)
	if err != nil {
		t.Fatal(err)
	}

	// 客户端公钥
	clientPublicKeyStr := "9m9lZaOf6ZHMjTbHNFviu+jzSa7p1sKuztW5k4FMXAA="
	clientPublicKey, err := base64.StdEncoding.DecodeString(clientPublicKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	clientPrivateKeyStr := "WHWFRw1swVA9jw4C9eRfOhyNqPZxsqswzi1fXLugaEA="
	clientPrivateKey, err := base64.StdEncoding.DecodeString(clientPrivateKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	// 服务端私钥
	servicePrivateKeyStr := "eTUAQMpb3+qbugvSVwxliNJyEJGhkd82ZLjyLF1tqKU="
	servicePrivateKey, err := base64.StdEncoding.DecodeString(servicePrivateKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	servicePublicKeyStr := "l1htdl2ndRCYnBYHZZ5v6GaGR2HZdyDr+sPxaDFOrgE="
	servicePublicKey, err := base64.StdEncoding.DecodeString(servicePublicKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	// 加密: 用服务端私钥和客户端公钥加密
	chaCha, err := NewChaChaPoly(servicePrivateKey, clientPublicKey, salt, sharedInfo, nil)
	if err != nil {
		t.Fatal(err)
	}

	originalText := "Hello, World!"

	ciphertext, err := chaCha.Encrypt([]byte(originalText))
	if err != nil {
		t.Fatal(err)
	}

	// 解密：用客户端私钥和服务端公钥解密
	chaCha1, err := NewChaChaPoly(clientPrivateKey, servicePublicKey, salt, sharedInfo, chaCha.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	decryptedText, err := chaCha1.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal([]byte(originalText), decryptedText) {
		t.Fatalf("Original text and decrypted text do not match. Original: %s, Decrypted: %s", originalText, string(decryptedText))
	}
}

// 测试客户端加密后的文本，服务端解密
func TestDecryption(t *testing.T) {
	// salt每次改变
	//salt := make([]byte, 16)
	//_, err := rand.Read(salt)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	salt := []byte("Talkify-Talkify")

	// auth是固定的
	sharedInfoStr := "5fuYp+O2f0mRxPMNFtryhw=="
	sharedInfo, err := base64.StdEncoding.DecodeString(sharedInfoStr)
	if err != nil {
		t.Fatal(err)
	}

	nonceStr := "/O0YzLUd4IrWA/dA"
	nonce, err := base64.StdEncoding.DecodeString(nonceStr)
	if err != nil {
		t.Fatal(err)
	}

	// 客户端公钥
	clientPublicKeyStr := "9m9lZaOf6ZHMjTbHNFviu+jzSa7p1sKuztW5k4FMXAA="
	clientPublicKey, err := base64.StdEncoding.DecodeString(clientPublicKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	// 这段文本是服务端加密的"Hello, World!"，然后base64
	ciphertext := "eXXZb8DYkokoAmi0XYWbUHrz9obXgD2KFYvCcEc="
	ciphertextData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	// 服务端私钥
	servicePrivateKeyStr := "eTUAQMpb3+qbugvSVwxliNJyEJGhkd82ZLjyLF1tqKU="
	servicePrivateKey, err := base64.StdEncoding.DecodeString(servicePrivateKeyStr)
	if err != nil {
		t.Fatal(err)
	}

	// 解密：用客户端私钥和服务端公钥解密
	chaCha1, err := NewChaChaPoly(servicePrivateKey, clientPublicKey, salt, sharedInfo, nonce)
	if err != nil {
		t.Fatal(err)
	}
	decryptedText, err := chaCha1.Decrypt(ciphertextData)
	if err != nil {
		t.Fatal(err)
	}
	// "Hello, World!"
	fmt.Println(decryptedText)
}
