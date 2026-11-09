package webtty

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

func decodeKey(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

const (
	loginSalt = "webterm-salt"
	loginInfo = "webterm-login"
)

// wireConnect is the websocket payload. Username and Password must stay empty;
// SSH credentials travel in Secret.
type wireConnect struct {
	Type     string `json:"type"`
	Mode     string `json:"mode"`
	Host     string `json:"host"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
	Key      string `json:"key,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
	Secret   string `json:"secret,omitempty"`
}

type loginBox struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func writeHello(conn wsConn, publicKey []byte) error {
	payload, err := json.Marshal(map[string]string{
		"type":      "hello",
		"publicKey": base64.StdEncoding.EncodeToString(publicKey),
	})
	if err != nil {
		return err
	}
	return conn.WriteMessage(textMessage, payload)
}

func decryptLogin(priv *ecdh.PrivateKey, keyB64, nonceB64, secretB64 string) (string, string, error) {
	if priv == nil || keyB64 == "" || nonceB64 == "" || secretB64 == "" {
		return "", "", errors.New("凭据必须加密传输")
	}
	clientRaw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	clientPub, err := ecdh.P256().NewPublicKey(clientRaw)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	shared, err := priv.ECDH(clientPub)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	aesKey, err := loginKey(shared)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return "", "", errors.New("凭据无法解密")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	var box loginBox
	if err = json.Unmarshal(plain, &box); err != nil {
		return "", "", errors.New("凭据无法解密")
	}
	return box.Username, box.Password, nil
}

func loginKey(shared []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, shared, []byte(loginSalt), loginInfo, 32)
}

// sealLogin encrypts credentials for tests and for any Go client.
func sealLogin(serverPub []byte, username, password string) (key, nonce, secret string, err error) {
	pub, err := ecdh.P256().NewPublicKey(serverPub)
	if err != nil {
		return "", "", "", err
	}
	client, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	shared, err := client.ECDH(pub)
	if err != nil {
		return "", "", "", err
	}
	aesKey, err := loginKey(shared)
	if err != nil {
		return "", "", "", err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonceRaw := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonceRaw); err != nil {
		return "", "", "", err
	}
	plain, err := json.Marshal(loginBox{Username: username, Password: password})
	if err != nil {
		return "", "", "", err
	}
	return base64.StdEncoding.EncodeToString(client.PublicKey().Bytes()),
		base64.StdEncoding.EncodeToString(nonceRaw),
		base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonceRaw, plain, nil)),
		nil
}
