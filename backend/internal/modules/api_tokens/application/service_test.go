package application

import (
	"reflect"
	"testing"
)

func TestNormalizeScopeCodesDeduplicatesAndLowercases(t *testing.T) {
	got := normalizeScopeCodes([]string{" message.write ", "MESSAGE.WRITE", "bot.write", ""})
	want := []string{"message.write", "bot.write"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeScopeCodes() = %#v, muốn %#v", got, want)
	}
}

func TestNewAPITokenReturnsHashWithoutLeakingToken(t *testing.T) {
	token, hash, err := newAPIToken()
	if err != nil {
		t.Fatalf("newAPIToken() trả lỗi: %v", err)
	}
	if token == "" || hash == "" {
		t.Fatal("token và hash không được rỗng")
	}
	if token == hash {
		t.Fatal("hash không được trùng token gốc")
	}
	if hashToken(token) != hash {
		t.Fatal("hash token không khớp")
	}
}

func TestHasScopeIsCaseInsensitive(t *testing.T) {
	if !hasScope([]string{"Message.Write"}, "message.write") {
		t.Fatal("hasScope() phải không phân biệt hoa thường")
	}
}
