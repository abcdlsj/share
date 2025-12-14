package main

import (
	"context"
	"flag"
	"log"
	"time"

	botapp "notionbot/internal/bot"
	"notionbot/internal/config"
	"notionbot/internal/notion"
	"notionbot/internal/storage"
	"notionbot/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path (optional)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = botAPI.Request(tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "begin", Description: "开始记录随笔"},
		tgbotapi.BotCommand{Command: "end", Description: "结束并输入标题"},
		tgbotapi.BotCommand{Command: "new", Description: "flush 当前并开启新随笔"},
		tgbotapi.BotCommand{Command: "cancel", Description: "取消并清空当前上下文"},
		tgbotapi.BotCommand{Command: "help", Description: "帮助"},
	))

	ctx := context.Background()
	s3u, err := storage.NewS3Uploader(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	loc, _ := time.LoadLocation(cfg.NotionTZ)
	nw := notion.NewWriter(cfg.NotionToken, cfg.NotionDatabase, cfg.NotionTitleProp, cfg.NotionCreatedProp, cfg.NotionVisibilityProp, cfg.NotionVisibilityValue, loc)
	st := store.NewStateStore()
	app := botapp.NewApp(botAPI, st, nw, s3u, cfg)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := botAPI.GetUpdatesChan(u)

	log.Printf("bot started as @%s", botAPI.Self.UserName)
	for upd := range updates {
		uctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		app.HandleUpdate(uctx, upd)
		cancel()
	}
}
