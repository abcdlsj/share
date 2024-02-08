package main

import (
	"kindle_bot/internal"
)

func main() {
	bot, err := internal.NewBot()
	if err != nil {
		panic(err)
	}
	err = bot.Serve()
	if err != nil {
		panic(err)
	}
}
