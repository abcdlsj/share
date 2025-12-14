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
	// Handle callback queries (inline keyboard button clicks)
	if upd.CallbackQuery != nil {
		a.handleCallback(ctx, upd.CallbackQuery)
		return
	}

	if upd.Message == nil {
		return
	}

	msg := upd.Message
	chatID := msg.Chat.ID
	st := a.store.Get(chatID)

	if msg.IsCommand() {
		a.handleCommand(ctx, st, chatID, msg.Command())
		return
	}

	switch st.Phase {
	case store.PhaseIdle:
		// Auto-start recording when content is received
		if msg.Text != "" || len(msg.Photo) > 0 {
			st.Phase = store.PhaseRecording
			st.Entries = nil
			st.EndedAt = time.Time{}
			if a.recordContent(ctx, chatID, st, msg) {
				a.replyWithKeyboard(chatID, "✓ Recorded", a.recordingKeyboard())
			}
			return
		}
		return
	case store.PhaseAwaitTitle:
		if title := strings.TrimSpace(msg.Text); title != "" {
			a.flushWithTitle(ctx, chatID, st, title)
			return
		}
		a.reply(chatID, "Please enter a title.")
		return
	case store.PhaseRecording:
		if a.recordContent(ctx, chatID, st, msg) {
			a.replyWithKeyboard(chatID, "✓ Recorded", a.recordingKeyboard())
		}
	}
}

func (a *App) recordContent(ctx context.Context, chatID int64, st *store.ChatContext, msg *tgbotapi.Message) bool {
	recorded := false

	if t := strings.TrimSpace(msg.Text); t != "" {
		st.Entries = append(st.Entries, model.Entry{Type: model.EntryText, Text: t})
		recorded = true
	}

	if len(msg.Photo) > 0 {
		url, err := a.handlePhoto(ctx, chatID, msg.Photo)
		if err != nil {
			a.reply(chatID, "Image failed: "+err.Error())
			return false
		}
		st.Entries = append(st.Entries, model.Entry{Type: model.EntryImage, URL: url})
		recorded = true
		if c := strings.TrimSpace(msg.Caption); c != "" {
			st.Entries = append(st.Entries, model.Entry{Type: model.EntryText, Text: c})
		}
	}

	return recorded
}

func (a *App) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID
	st := a.store.Get(chatID)

	// Acknowledge the callback
	callback := tgbotapi.NewCallback(cq.ID, "")
	_, _ = a.bot.Request(callback)

	a.handleCommand(ctx, st, chatID, cq.Data)
}

func (a *App) handleCommand(ctx context.Context, st *store.ChatContext, chatID int64, cmd string) {
	switch cmd {
	case "start", "help":
		a.replyWithKeyboard(chatID, "Select action:", a.idleKeyboard())
		return
	case "begin":
		st.Phase = store.PhaseRecording
		st.Entries = nil
		st.EndedAt = time.Time{}
		a.replyWithKeyboard(chatID, "Recording started.", a.recordingKeyboard())
		return
	case "end":
		if st.Phase != store.PhaseRecording {
			a.replyWithKeyboard(chatID, "Not recording.", a.idleKeyboard())
			return
		}
		st.Phase = store.PhaseAwaitTitle
		st.EndedAt = time.Now()
		a.reply(chatID, "Enter title:")
		return
	case "new":
		if st.Phase != store.PhaseRecording {
			st.Phase = store.PhaseRecording
			st.Entries = nil
			a.replyWithKeyboard(chatID, "New note started.", a.recordingKeyboard())
			return
		}
		if len(st.Entries) > 0 {
			a.flushWithTitle(ctx, chatID, st, defaultTitle())
		}
		st.Phase = store.PhaseRecording
		st.Entries = nil
		a.replyWithKeyboard(chatID, "Flushed. New note started.", a.recordingKeyboard())
		return
	case "cancel":
		st.Phase = store.PhaseIdle
		st.Entries = nil
		st.EndedAt = time.Time{}
		a.replyWithKeyboard(chatID, "Cancelled.", a.idleKeyboard())
		return
	default:
		return
	}
}

func (a *App) idleKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ Begin", "begin"),
		),
	)
}

func (a *App) recordingKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏹ End", "end"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 New", "new"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel"),
		),
	)
}

func (a *App) flushWithTitle(ctx context.Context, chatID int64, st *store.ChatContext, title string) {
	entries := append([]model.Entry(nil), st.Entries...)
	if len(entries) == 0 {
		a.reply(chatID, "No content to save.")
		st.Phase = store.PhaseIdle
		return
	}

	_, pageURL, err := a.notion.CreateNotePage(ctx, title, entries)
	if err != nil {
		a.reply(chatID, "Notion error: "+err.Error())
		return
	}

	st.Phase = store.PhaseIdle
	st.Entries = nil
	st.EndedAt = time.Time{}
	a.reply(chatID, "Saved: "+pageURL)
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

func (a *App) replyWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	_, _ = a.bot.Send(msg)
}

func defaultTitle() string {
	loc := time.FixedZone("CST", 8*3600)
	return "Note " + time.Now().In(loc).Format("2006-01-02 15:04")
}
