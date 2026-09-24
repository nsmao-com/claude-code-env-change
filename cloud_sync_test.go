package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"os"
	"path/filepath"
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

// 换电脑恢复：备份里的 MCP 与 Skills 必须写回各平台文件，且重新加载后平台标记仍在；
// 新电脑上本来就有、备份里没有的 MCP 服务器不能被删掉
func TestCloudRestoreWritesBackToPlatforms(t *testing.T) {
	home, _ := withMcpHome(t)
	claudeJSON := filepath.Join(home, claudeMcpFile)
	if err := os.WriteFile(claudeJSON, []byte(`{"mcpServers":{"local-only":{"command":"node","args":["x.js"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	ms := NewMCPService()
	ss := NewSkillService()
	cs := &CloudSyncService{mcp: ms, skills: ss}
	bundle := cloudBundle{Version: 1, Files: map[string]json.RawMessage{
		"mcp.json":    json.RawMessage(`{"fetch":{"type":"stdio","command":"uvx","args":["mcp-server-fetch"],"enable_platform":["claude-code","codex"]}}`),
		"skills.json": json.RawMessage(`{"review":{"content":"---\nname: review\ndescription: d\n---\nbody","enable_platform":["claude-code"]}}`),
	}}
	msg, err := cs.restoreBundle(bundle)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if strings.Contains(msg, "⚠") {
		t.Fatalf("写回平台不应失败: %s", msg)
	}

	servers := readJSONFile(t, claudeJSON)["mcpServers"].(map[string]any)
	if _, ok := servers["fetch"]; !ok {
		t.Fatalf("fetch 应写回 Claude Code: %v", servers)
	}
	if _, ok := servers["local-only"]; !ok {
		t.Fatalf("本机已有的 local-only 不能被恢复流程删掉: %v", servers)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err != nil || !strings.Contains(string(data), "fetch") {
		t.Fatalf("fetch 应写回 Codex: %v %s", err, data)
	}
	if !fileExists(filepath.Join(home, ".claude", "skills", "review", "SKILL.md")) {
		t.Fatalf("Skill 应写回 ~/.claude/skills")
	}

	listed, err := ms.ListServers()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range listed {
		if s.Name == "fetch" && !(platformContains(s.EnablePlatform, platClaudeCode) && platformContains(s.EnablePlatform, platCodex)) {
			t.Fatalf("重新加载后 fetch 的平台标记丢失: %v", s.EnablePlatform)
		}
	}
	skills, err := ss.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || !platformContains(skills[0].EnablePlatform, platClaudeCode) {
		t.Fatalf("重新加载后 Skill 的平台标记丢失: %+v", skills)
	}
}

// 对象 Key 按 SigV4 UriEncode 编码，且实际请求路径与签名用的 canonical URI 一致
func TestOSSKeyEncodingMatchesCanonicalURI(t *testing.T) {
	if got := encodeOSSPath("dir one/backup+v2@home=1.bin"); got != "dir%20one/backup%2Bv2%40home%3D1.bin" {
		t.Fatalf("编码不符合 SigV4 UriEncode: %s", got)
	}
	if got := encodeOSSPath("备份/a~b_c-d.bin"); got != "%E5%A4%87%E4%BB%BD/a~b_c-d.bin" {
		t.Fatalf("非 ASCII 与保留字符编码不正确: %s", got)
	}
	c := newOSSObjectClient(CloudConfig{Provider: "s3", Region: "us-east-1", Bucket: "b", AccessKey: "ak", SecretKey: "sk"}, nil)
	req, err := c.newRequest("GET", "k+1.bin", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.URL.EscapedPath(); got != "/k%2B1.bin" {
		t.Fatalf("实际发送的路径应与签名一致，得到 %s", got)
	}
}
