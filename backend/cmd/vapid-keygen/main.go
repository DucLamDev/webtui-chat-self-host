package main

import (
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not generate VAPID keys:", err)
		os.Exit(1)
	}
	fmt.Println("WEB_PUSH_VAPID_PUBLIC_KEY=" + publicKey)
	fmt.Println("WEB_PUSH_VAPID_PRIVATE_KEY=" + privateKey)
}
