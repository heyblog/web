package security

import "testing"

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"ok":true}`)
	header := Sign("secret", body)

	if !Verify("secret", header, body) {
		t.Fatal("expected valid signature")
	}
	if Verify("secret", header, []byte(`{"ok":false}`)) {
		t.Fatal("expected invalid signature")
	}
}
