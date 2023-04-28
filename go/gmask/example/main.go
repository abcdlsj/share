package main

import "fmt"

func main() {
	mPhone := "12345678901"                                  // MASK: `match` `12345678901` `***********`
	mEmail, nEmail := "112233@gmail.com", "223344.gmail.com" // MASK: `regexp` `"[^"]*"` `"*********"`

	fmt.Println("mPhone:", mPhone)
	fmt.Println("mEmail:", mEmail)
	fmt.Println("nEmail:", nEmail)
}
