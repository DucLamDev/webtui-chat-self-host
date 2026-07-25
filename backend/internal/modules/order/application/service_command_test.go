package application

import "testing"

func TestParseAutoBotCommandRecognizesQuickOrderPayment(t *testing.T) {
	command := parseAutoBotCommand("Tạo QR cho đơn hàng QOIABCD1234EFGH5678")

	if !command.HasOrderPayment {
		t.Fatal("expected Quick Order payment intent")
	}
	if command.IntentCode != "QOIABCD1234EFGH5678" {
		t.Fatalf("IntentCode = %q", command.IntentCode)
	}
}

func TestParseAutoBotCommandKeepsWalletDepositSeparate(t *testing.T) {
	command := parseAutoBotCommand("Email: khach@example.com\nSố tiền: 200000")

	if command.HasOrderPayment {
		t.Fatal("wallet deposit must not be parsed as an order payment")
	}
	if !command.HasAmount || command.Amount != 200000 {
		t.Fatalf("Amount = %d, HasAmount = %v", command.Amount, command.HasAmount)
	}
}

func TestParseAutoBotCommandRecognizesRenewalByServiceID(t *testing.T) {
	command := parseAutoBotCommand("Tôi muốn gia hạn dịch vụ VPS #1234 của tài khoản khach@example.com thêm 3 tháng.")

	if !command.RenewalIntent {
		t.Fatal("expected renewal intent")
	}
	if command.Email != "khach@example.com" || command.ServiceID != 1234 || command.ServiceType != "vps" || command.Months != 3 {
		t.Fatalf("command = %#v", command)
	}
}

func TestParseAutoBotCommandRecognizesRenewalByServiceName(t *testing.T) {
	command := parseAutoBotCommand("Gia hạn dịch vụ vps-hanoi-01 của tài khoản khach@example.com trong 2 tháng")

	if !command.RenewalIntent {
		t.Fatal("expected renewal intent")
	}
	if command.ServiceName != "vps-hanoi-01" || command.ServiceType != "vps" || command.Months != 2 {
		t.Fatalf("command = %#v", command)
	}
}

func TestParseAutoBotCommandKeepsExpiringStatisticsSeparateFromRenewal(t *testing.T) {
	command := parseAutoBotCommand("Thống kê dịch vụ cần gia hạn của khach@example.com trong 30 ngày")

	if command.RenewalIntent {
		t.Fatal("statistics request must not trigger a chargeable renewal")
	}
	if !command.HasLookup || command.Days != 30 {
		t.Fatalf("command = %#v", command)
	}
}

func TestParseAutoBotCommandRecognizesNaturalWalletDeposit(t *testing.T) {
	command := parseAutoBotCommand("Nạp ví 500k cho tài khoản khach@example.com")

	if command.HasOrderPayment {
		t.Fatal("wallet deposit must stay separate from order payment")
	}
	if !command.PaymentIntent || command.Email != "khach@example.com" || command.Amount != 500000 {
		t.Fatalf("command = %#v", command)
	}
}
