package bot

import (
	"context"
	"os"
	"strings"
	"time"

	"notionbot/internal/config"
	"notionbot/internal/imageutil"
	"notionbot/internal/model"
	"notionbot/internal/notion"
	"notionbot/internal/storage"
	"notionbot/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type App struct {
	bot    *tgbotapi.BotAPI
	store  *store.StateStore
	notion *notion.Writer
	s3     *storage.S3Uploader
	cfg    config.Config
}

func NewApp(botAPI *tgbotapi.BotAPI, st *store.StateStore, nw *notion.Writer, s3u *storage.S3Uploader, cfg config.Config) *App {
	return &App{bot: botAPI, store: st, notion: nw, s3: s3u, cfg: cfg}
}

func (a *App) HandleUpdate(ctx context.Context, upd tgbotapi.Update) {
	if upd.Message == nil {
		return
	}

	msg := upd.Message
	chatID := msg.Chat.ID
	st := a.store.Get(chatID)

	if msg.IsCommand() {
		a.handleCommand(ctx, st, msg, msg.Command())
		return
	}
	if msg.Text != "" {
		switch strings.TrimSpace(msg.Text) {
		case "开始":
			a.handleCommand(ctx, st, msg, "begin")
			return
		case "结束":
			a.handleCommand(ctx, st, msg, "end")
			return
		case "新随笔", "开启新随笔":
			a.handleCommand(ctx, st, msg, "new")
			return
		case "取消":
			a.handleCommand(ctx, st, msg, "cancel")
			return
		}
	}

	switch st.Phase {
	case store.PhaseIdle:
		return
	case store.PhaseAwaitTitle:
		if title := strings.TrimSpace(msg.Text); title != "" {
			a.flushWithTitle(ctx, chatID, st, title)
			return
		}
		a.reply(chatID, "请输入标题（纯文本）。")
		return
	case store.PhaseRecording:
		if t := strings.TrimSpace(msg.Text); t != "" {
			st.Entries = append(st.Entries, model.Entry{Type: model.EntryText, Text: t})
		}

		if len(msg.Photo) > 0 {
			url, err := a.handlePhoto(ctx, chatID, msg.Photo)
			if err != nil {
				a.reply(chatID, "图片处理失败："+err.Error())
				return
			}
			st.Entries = append(st.Entries, model.Entry{Type: model.EntryImage, URL: url})
			if c := strings.TrimSpace(msg.Caption); c != "" {
				st.Entries = append(st.Entries, model.Entry{Type: model.EntryText, Text: c})
			}
		}
	}
}

func (a *App) handleCommand(ctx context.Context, st *store.ChatContext, msg *tgbotapi.Message, cmd string) {
	chatID := msg.Chat.ID

	switch cmd {
	case "start", "help":
		a.reply(chatID, "命令：/begin 开始记录，/end 结束并输入标题，/new flush 并开启新随笔。也支持直接发“开始/结束/新随笔”。")
		return
	case "begin":
		st.Phase = store.PhaseRecording
		st.Entries = nil
		st.EndedAt = time.Time{}
		a.reply(chatID, "开始记录。直到 /end 之前的消息都会写入同一篇随笔。")
		return
	case "end":
		if st.Phase != store.PhaseRecording {
			a.reply(chatID, "当前不在记录中。用 /begin 开始。")
			return
		}
		st.Phase = store.PhaseAwaitTitle
		st.EndedAt = time.Now()
		a.reply(chatID, "已结束。请输入这篇随笔的标题（下一条消息作为标题）。")
		return
	case "new":
		if st.Phase != store.PhaseRecording {
			st.Phase = store.PhaseRecording
			st.Entries = nil
			a.reply(chatID, "已开启新随笔（当前为空）。")
			return
		}
		if len(st.Entries) > 0 {
			a.flushWithTitle(ctx, chatID, st, defaultTitle())
		}
		st.Phase = store.PhaseRecording
		st.Entries = nil
		a.reply(chatID, "已 flush 并开启新随笔。")
		return
	case "cancel":
		st.Phase = store.PhaseIdle
		st.Entries = nil
		st.EndedAt = time.Time{}
		a.reply(chatID, "已取消并清空当前上下文。")
		return
	default:
		return
	}
}

func (a *App) flushWithTitle(ctx context.Context, chatID int64, st *store.ChatContext, title string) {
	entries := append([]model.Entry(nil), st.Entries...)
	if len(entries) == 0 {
		a.reply(chatID, "当前没有内容可写入 Notion。")
		st.Phase = store.PhaseIdle
		return
	}

	_, pageURL, err := a.notion.CreateNotePage(ctx, title, entries)
	if err != nil {
		a.reply(chatID, "写入 Notion 失败："+err.Error())
		return
	}

	st.Phase = store.PhaseIdle
	st.Entries = nil
	st.EndedAt = time.Time{}
	a.reply(chatID, "已写入 Notion："+pageURL)
}

func (a *App) handlePhoto(ctx context.Context, chatID int64, photos []tgbotapi.PhotoSize) (string, error) {
	p := photos[len(photos)-1]
	file, err := a.bot.GetFile(tgbotapi.FileConfig{FileID: p.FileID})
	if err != nil {
		return "", err
	}

	srcPath, size, contentType, err := imageutil.DownloadToTempFile(ctx, file.Link(a.cfg.TelegramToken), a.cfg.TelegramDownloadMaxBytes)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(srcPath) }()

	upPath := srcPath
	upContentType := contentType
	if size > a.cfg.MaxImageBytes {
		outPath, _, err := imageutil.CompressToJPEGUnder(srcPath, a.cfg.MaxImageBytes, a.cfg.ImgJPEGMinQuality)
		if err != nil {
			return "", err
		}
		defer func() { _ = os.Remove(outPath) }()
		upPath = outPath
		upContentType = "image/jpeg"
	}

	f, err := os.Open(upPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	publicURL, err := a.s3.UploadPublic(ctx, chatID, f, upContentType)
	if err != nil {
		return "", err
	}
	return publicURL, nil
}

func (a *App) reply(chatID int64, text string) {
	_, _ = a.bot.Send(tgbotapi.NewMessage(chatID, text))
}

func defaultTitle() string {
	loc := time.FixedZone("CST", 8*3600)
	return "随笔 " + time.Now().In(loc).Format("2006-01-02 15:04")
}
