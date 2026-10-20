package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSnapshotContractRejectsDigestHeaderAndDuplicates(t *testing.T) {
	header := strings.Join(Columns, ",") + "\n"
	row := "eml_00000000000000000000000000000001,c6s," + strings.Repeat(`\x`+strings.Repeat("a", 64)+",", 3) + "cerberus.beta-invitation,failed,1,2026-10-20 00:00:00+00,2026-10-20 00:00:00+00,\n"
	check := func(data string) (int, error) {
		h := sha256.Sum256([]byte(data))
		return Validate([]byte(data), hex.EncodeToString(h[:]))
	}
	if n, err := check(header + row); err != nil || n != 1 {
		t.Fatalf("valid snapshot: %v", err)
	}
	for _, data := range []string{header + row + row, "wrong,header\n", header + strings.Replace(row, `\x`, "", 1)} {
		if _, err := check(data); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
	if _, err := Validate([]byte(header+row), strings.Repeat("0", 64)); err == nil {
		t.Fatal("digest mismatch accepted")
	}
}
