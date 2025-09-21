package security

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"edgeflow/pkg/logger"
	"errors"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"io"
	"log"
)

type ChaChaPoly struct {
	senderPrivateKey, // 发送方的私钥
	receiverPublicKey, // 接收方的公钥
	salt, // 加盐，保持加密和解密时是确定的
	sharedInfo, // 鉴权，保持一直固定的
	_symmetricKey, // 生成的对称密钥
	Nonce []byte
	aead cipher.AEAD // aead 解密解密实例
}

func NewChaChaPoly(senderPrivateKey, receiverPublicKey, salt, sharedInfo, nonce []byte) (*ChaChaPoly, error) {
	if len(senderPrivateKey) == 0 || len(receiverPublicKey) == 0 {
		return nil, errors.New("Key is not empty")
	}
	chaCha := &ChaChaPoly{
		senderPrivateKey:  senderPrivateKey,
		receiverPublicKey: receiverPublicKey,
		salt:              salt,
		sharedInfo:        sharedInfo,
		Nonce:             nonce,
	}
	// 生成对成密钥
	key, err := chaCha.symmetricKey()
	if err != nil {
		log.Println("衍生密钥失败")
		logger.Errorf("衍生密钥失败")
		return nil, err
	}

	// 初始化AEAD加密实例，New和NewX处理的nonce（number used once）大小是不同的
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	chaCha.aead = aead
	if chaCha.Nonce == nil {
		nonce, err = chaCha.makeNonce()
		if err != nil {
			return nil, err
		}
		chaCha.Nonce = nonce
	}
	return chaCha, nil
}

// 密钥衍生：
// deriveKey函数的作用是将给定的共享密钥、盐（salt）和其他共享信息（sharedInfo）通过某种KDF（例如HKDF）转换为可用于实际加密和解密的对称密钥。
func (c *ChaChaPoly) deriveKey(sharedSecret, salt, sharedInfo []byte) ([]byte, error) {
	// 1. 使用给定的共享密钥、盐和其他共享信息初始化KDF
	hkdfSha512 := hkdf.New(sha512.New, sharedSecret, salt, sharedInfo)
	// 2. 使用KDF生成一个或多个子密钥
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(hkdfSha512, key); err != nil {
		return nil, err
	}
	// 3. 返回衍生出的密钥
	return key, nil
}

// 生成共享密钥
func (c *ChaChaPoly) generateSharedSecret() ([]byte, error) {
	var sharedSecret, priv, pub [32]byte
	copy(priv[:], c.senderPrivateKey)
	copy(pub[:], c.receiverPublicKey)
	curve25519.ScalarMult(&sharedSecret, &priv, &pub)
	return sharedSecret[:], nil
}

// 生成对成密钥
func (c *ChaChaPoly) symmetricKey() ([]byte, error) {
	if c._symmetricKey != nil {
		return c._symmetricKey, nil
	}
	// 1.生成共享密钥
	// 根据服务端的私钥和客户端的公钥匙 -》生成共享密钥
	sharedSecret, err := c.generateSharedSecret()
	if err != nil {
		logger.Errorf("生成共享密钥失败！")
		return nil, err
	}

	// 密码衍生
	// 将共享密钥转化为一个实际用于加密和解密的对称密钥
	key, err := c.deriveKey(sharedSecret, c.salt, c.sharedInfo)
	if err != nil {
		logger.Errorf("衍生密钥失败")
		return nil, err
	}
	c._symmetricKey = key
	return key, nil
}

func (c *ChaChaPoly) makeNonce() ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}

// 加密
func (c *ChaChaPoly) Encrypt(plaintext []byte) ([]byte, error) {

	ciphertext := c.aead.Seal(nil, c.Nonce, plaintext, nil)
	return ciphertext, nil
}

// 解密
func (c *ChaChaPoly) Decrypt(ciphertext []byte) ([]byte, error) {

	plaintext, err := c.aead.Open(nil, c.Nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// 生成32位的私钥，与客户端保持一致
func GenCurve25519Key() (privateKey, publicKey []byte, err error) {
	privateKey = make([]byte, 32)
	_, err = rand.Read(privateKey)
	if err != nil {
		return
	}

	// 根据私钥获取公钥
	publicKey, err = curve25519.X25519(privateKey, curve25519.Basepoint)
	return
}
