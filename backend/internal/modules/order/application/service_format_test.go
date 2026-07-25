package application

import (
	"strings"
	"testing"
)

func TestFormatDepositQRMessageDoesNotExposeImageURL(t *testing.T) {
	body := formatDepositQRMessage(WalletDepositQRData{
		Amount:    200000,
		Email:     "khach@example.com",
		QRURL:     "https://order.example/qr/private-token.png",
		Reference: "WQR-123",
	})

	if strings.Contains(body, "https://order.example") || strings.Contains(body, "QR:") {
		t.Fatalf("QR URL must stay in message metadata, got %q", body)
	}
	if !strings.Contains(body, "THANH TOÁN · QR NẠP VÍ") {
		t.Fatalf("expected professional response heading, got %q", body)
	}
}

func TestFormatOrderPaymentQRMessageDoesNotExposeImageURL(t *testing.T) {
	body := formatOrderPaymentQRMessage(OrderPaymentQRData{
		Amount:          350000,
		ExternalOrderID: "QO-123",
		QRURL:           "https://order.example/qr/order-token.png",
		Reference:       "PAY-123",
	})

	if strings.Contains(body, "https://order.example") || strings.Contains(body, "QR:") {
		t.Fatalf("QR URL must stay in message metadata, got %q", body)
	}
	if !strings.Contains(body, "THANH TOÁN · QR ĐƠN HÀNG") {
		t.Fatalf("expected professional response heading, got %q", body)
	}
}

func TestFormatRenewServiceMessageShowsShortageAndSupportRoute(t *testing.T) {
	body := formatRenewServiceMessage(RenewServiceData{
		Outcome:        "insufficient_balance",
		ServiceType:    "VPS",
		ServiceID:      1234,
		ServiceName:    "vps-hanoi-01",
		Months:         1,
		Amount:         500000,
		BalanceBefore:  350000,
		ShortageAmount: 150000,
	})

	for _, expected := range []string{"SỐ DƯ KHÔNG ĐỦ", "150.000 VND", "Zalo OA VPSTTT", "#ke-toan"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body = %q, want %q", body, expected)
		}
	}
}

func TestFormatRenewServiceMessageShowsSuccessfulRenewal(t *testing.T) {
	body := formatRenewServiceMessage(RenewServiceData{
		Outcome:        "renewed",
		TransactionID:  "REN-123",
		ServiceType:    "VPS",
		ServiceID:      1234,
		ServiceName:    "vps-hanoi-01",
		Months:         1,
		Amount:         500000,
		BalanceBefore:  900000,
		BalanceAfter:   400000,
		ExpiresAtAfter: "2026-08-20",
	})

	for _, expected := range []string{"ĐÃ HOÀN TẤT", "REN-123", "400.000 VND", "2026-08-20"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body = %q, want %q", body, expected)
		}
	}
}
