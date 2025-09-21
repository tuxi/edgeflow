package security

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
)

type Rsa struct {
	publicKeyPath  string
	privateKeyPath string
	publicKey      *rsa.PublicKey
	privateKey     *rsa.PrivateKey
}

func NewRsa(publicKeyPath, privateKeyPath string) *Rsa {
	return &Rsa{
		publicKeyPath:  publicKeyPath,
		privateKeyPath: privateKeyPath,
	}
}

// 公钥加密
// plainText 要加密的数据
// publicKeyPath 公钥匙文件地址
func (r *Rsa) Encrypt(plainText []byte) ([]byte, error) {
	//pem解码
	if r.publicKey == nil {
		err := r.DecodePublicKey()
		if err != nil {
			return nil, err
		}
	}
	//对明文进行加密
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, r.publicKey, plainText)
	if err != nil {
		panic(err)
	}
	//返回密文
	return cipherText, nil
}

// 公钥分段加密，加密较大数据
// plainText 要加密的数据
// publicKeyPath 公钥匙文件地址
func (r *Rsa) EncryptBlock(plainText []byte) ([]byte, error) {
	if r.publicKey == nil {
		err := r.DecodePublicKey()
		if err != nil {
			return nil, err
		}
	}
	keySize, srcSize := r.publicKey.Size(), len(plainText)
	pub := r.publicKey
	//logs.Debug("密钥长度：", keySize, "\t明文长度：\t", srcSize)
	//单次加密的长度需要减掉padding的长度，PKCS1为11
	offSet, once := 0, keySize-11
	buffer := bytes.Buffer{}
	for offSet < srcSize {
		endIndex := offSet + once
		if endIndex > srcSize {
			endIndex = srcSize
		}
		// 加密一部分
		bytesOnce, err := rsa.EncryptPKCS1v15(rand.Reader, pub, plainText[offSet:endIndex])
		if err != nil {
			return nil, err
		}
		buffer.Write(bytesOnce)
		offSet = endIndex
	}
	return buffer.Bytes(), nil
}

// 私钥解密
// cipherText 需要解密的byte数据
// path 私钥文件路径
func (r *Rsa) Decrypt(cipherText []byte) ([]byte, error) {
	// 解码私钥
	if r.privateKey == nil {
		err := r.DecodePrivateKey()
		if err != nil {
			return nil, err
		}
	}

	//对密文进行解密
	plainText, err := rsa.DecryptPKCS1v15(rand.Reader, r.privateKey, cipherText)
	//返回明文
	return plainText, err
}

// 私钥分段解密，解密较大数据
// cipherText 需要解密的byte数据
// path 私钥文件路径
func (r *Rsa) DecryptBlock(cipherText []byte) ([]byte, error) {
	//pem解码
	if r.privateKey == nil {
		err := r.DecodePrivateKey()
		if err != nil {
			return nil, err
		}
	}
	//对密文进行解密
	private := r.privateKey
	keySize, srcSize := private.Size(), len(cipherText)
	//logs.Debug("密钥长度：", keySize, "\t密文长度：\t", srcSize)
	var offSet = 0
	var buffer = bytes.Buffer{}
	for offSet < srcSize {
		endIndex := offSet + keySize
		if endIndex > srcSize {
			endIndex = srcSize
		}
		bytesOnce, err := rsa.DecryptPKCS1v15(rand.Reader, private, cipherText[offSet:endIndex])
		if err != nil {
			return nil, err
		}
		buffer.Write(bytesOnce)
		offSet = endIndex
	}
	plainText := buffer.Bytes()
	//返回明文
	return plainText, nil
}

// 私钥分段解密，解密较大数据
// cipherText 需要解密的byte数据
// path 私钥文件路径
func (r *Rsa) DecryptBlockString(cipherBase64 string) ([]byte, error) {
	cipherText, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return nil, nil
	}
	plainBytes, err := r.DecryptBlock(cipherText)
	//返回明文
	return plainBytes, err
}

// pem解码
func (r *Rsa) DecodePublicKey() error {
	if len(r.publicKeyPath) == 0 {
		return errors.New("PublicKeyPath is not exist")
	}
	//打开文件
	file, err := os.Open(r.publicKeyPath)
	if err != nil {
		return err
	}
	defer file.Close()
	//读取文件的内容
	info, _ := file.Stat()
	buf := make([]byte, info.Size())
	_, err = file.Read(buf)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(buf)
	//x509解码

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	//类型断言
	publicKey := publicKeyInterface.(*rsa.PublicKey)
	r.publicKey = publicKey
	return nil
}

func (r *Rsa) DecodePrivateKey() error {
	if len(r.privateKeyPath) == 0 {
		return errors.New("PrivateKeyPath is not exist")
	}
	file, err := os.Open(r.privateKeyPath)
	if err != nil {
		return err
	}
	defer file.Close()
	//获取文件内容
	info, _ := file.Stat()
	buf := make([]byte, info.Size())
	_, err = file.Read(buf)
	if err != nil {
		return err
	}
	//pem解码
	block, _ := pem.Decode(buf)
	//X509解码
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	r.privateKey = privateKey
	return nil
}
