package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"strings"
	"testing"
)

func TestCloudPayloadRoundTrip(t *testing.T) {
	plain := []byte(`{"version":1,"files":{"config.json":{"environments":[]}}}`)
	enc, err := encryptCloudPayload(plain, "correct horse")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if string(enc[:4]) != cloudMagic {
		t.Fatalf("新备份应使用 %s 格式，实际 %q", cloudMagic, enc[:4])
	}
	if bytes.Contains(enc, []byte("environments")) {
		t.Fatalf("密文里出现了明文内容")
	}

	got, err := decryptCloudPayload(enc, "correct horse")
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("解密结果不一致: %s", got)
	}

	if _, err := decryptCloudPayload(enc, "wrong"); err == nil {
		t.Fatalf("错误口令应解密失败")
	}
	if _, err := decryptCloudPayload(enc, ""); err == nil {
		t.Fatalf("未填口令时应提示需要口令")
	}
}

// 旧版本（CEB1）上传的备份必须仍能恢复
func TestCloudPayloadDecryptsLegacyFormat(t *testing.T) {
	plain := []byte(`{"version":1,"files":{}}`)
	salt := bytes.Repeat([]byte{7}, 16)
	nonce := bytes.Repeat([]byte{9}, 12)
	block, err := aes.NewCipher(deriveLegacyCloudKey([]byte("old-pass"), salt))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(cloudMagicLegacy), salt...)
	raw = append(raw, nonce...)
	raw = append(raw, gcm.Seal(nil, nonce, plain, nil)...)

	got, err := decryptCloudPayload(raw, "old-pass")
	if err != nil {
		t.Fatalf("旧格式备份解密失败: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("旧格式解密结果不一致: %s", got)
	}
}

// 本机设了口令时，云端出现明文备份说明可能被篡改，必须拒绝；未设口令时照旧可用
func TestCloudPayloadRejectsPlaintextWhenPassphraseSet(t *testing.T) {
	plain := []byte(`{"version":1,"files":{}}`)
	if _, err := decryptCloudPayload(plain, "secret"); err == nil || !strings.Contains(err.Error(), "未加密") {
		t.Fatalf("设置口令时应拒绝明文备份，实际 err=%v", err)
	}
	got, err := decryptCloudPayload(plain, "")
	if err != nil {
		t.Fatalf("未设置口令时明文备份应可用: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("明文结果不一致")
	}
}

// 把新格式的 magic 改成旧格式，不能借旧 KDF 绕过（magic 参与 GCM 认证）
func TestCloudPayloadMagicDowngradeFails(t *testing.T) {
	enc, err := encryptCloudPayload([]byte(`{"files":{}}`), "pass")
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(cloudMagicLegacy), enc[4:]...)
	if _, err := decryptCloudPayload(tampered, "pass"); err == nil {
		t.Fatalf("篡改 magic 后应解密失败")
	}
}
