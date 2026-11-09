package webtty

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"os/exec"
	"testing"
)

func TestWebCryptoCompatible(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())
	script := `
const crypto = globalThis.crypto;
const pub = process.argv[1];
function bytesToBase64(bytes) {
  return Buffer.from(bytes).toString("base64");
}
function base64ToBytes(value) {
  return new Uint8Array(Buffer.from(value, "base64"));
}
(async () => {
  const serverKey = await crypto.subtle.importKey("raw", base64ToBytes(pub), { name: "ECDH", namedCurve: "P-256" }, false, []);
  const client = await crypto.subtle.generateKey({ name: "ECDH", namedCurve: "P-256" }, true, ["deriveBits"]);
  const shared = await crypto.subtle.deriveBits({ name: "ECDH", public: serverKey }, client.privateKey, 256);
  const material = await crypto.subtle.importKey("raw", shared, "HKDF", false, ["deriveKey"]);
  const aesKey = await crypto.subtle.deriveKey(
    { name: "HKDF", hash: "SHA-256", salt: new TextEncoder().encode("webterm-salt"), info: new TextEncoder().encode("webterm-login") },
    material,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt"]
  );
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const plain = new TextEncoder().encode(JSON.stringify({ username: "root", password: "s3cret" }));
  const secret = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, aesKey, plain));
  const clientPub = new Uint8Array(await crypto.subtle.exportKey("raw", client.publicKey));
  process.stdout.write([bytesToBase64(clientPub), bytesToBase64(nonce), bytesToBase64(secret)].join("\n"));
})().catch((err) => { console.error(err); process.exit(1); });
`
	out, err := exec.Command("node", "-e", script, pub).Output()
	if err != nil {
		t.Fatal(err)
	}
	parts := splitLines(string(out))
	if len(parts) != 3 {
		t.Fatalf("node output %q", out)
	}
	user, pass, err := decryptLogin(priv, parts[0], parts[1], parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if user != "root" || pass != "s3cret" {
		t.Fatalf("got %s %s", user, pass)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	return lines
}
