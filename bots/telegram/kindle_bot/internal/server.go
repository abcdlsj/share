package internal

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
)

const (
	OnStart    = "OnStart"
	OnSetEmail = "OnSetEmail"
)

type kindleCfg struct {
	DebugTrace  bool
	KindleEmail string
}

const debugTrace = true

type UserState struct {
	mu    sync.Mutex
	State map[int64]string
}

var GlobalUserState = UserState{
	State: make(map[int64]string),
}

func init() {
	logFile, err := os.Create("output.log")
	if err != nil {
		panic(err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
}

type Bot struct {
	mu      sync.Mutex
	Backend *tele.Bot
	Cfg     *kindleCfg
	History map[time.Time]history
}

type history struct {
	Result   string
	Detail   string
	FileName string
}

func (k *Bot) SetHandler() error {
	if debugTrace {
		k.Backend.Use(middleware.Logger())
	}
	k.Backend.Use(checkSender([]int64{******}...)) // MASK_DONE
	k.Backend.Handle(tele.OnText, k.StateHandler)
	k.Backend.Handle("/start", k.StartHandler)
	k.Backend.Handle("/history", k.HistoryHandler)
	k.Backend.Handle(tele.OnDocument, k.SendFileHandler)
	return nil
}

func NewBot() (*Bot, error) {
	bot, err := tele.NewBot(tele.Settings{
		Token:  "******", // MASK_DONE
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return &Bot{
		Backend: bot,
		Cfg:     &kindleCfg{},
		History: make(map[time.Time]history),
	}, nil
}

func checkSender(chats ...int64) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return middleware.Restrict(middleware.RestrictConfig{
			Chats: chats,
			In:    next,
			Out: func(c tele.Context) error {
				_ = c.Send("you can not use this bot, pls contact admin")
				if debugTrace {
					log.Errorf("user %d try to use bot, but whitlist chats: [%+v]", c.Sender().ID, chats)
				}
				return nil
			},
		})(next)
	}
}

func (k *Bot) Serve() error {
	if err := k.SetHandler(); err != nil {
		return err
	}
	k.Backend.Start()
	return nil
}

func (k *Bot) StateHandler(c tele.Context) error {
	// check user state
	userState := GlobalUserState.State[c.Sender().ID]
	switch userState {
	case OnSetEmail:
		k.Cfg.KindleEmail = c.Message().Text
		_, _ = k.Backend.Send(c.Sender(), fmt.Sprintf("set kindle email to %s", k.Cfg.KindleEmail))
		setUserState(c.Sender().ID, OnStart)
	default:
		_, _ = k.Backend.Send(c.Sender(), "pls send file to me\nif you want to change email, pls use /start")
	}

	return nil
}

func (k *Bot) HistoryHandler(c tele.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	var msg string
	for k, v := range k.History {
		msg += fmt.Sprintf("%s %s %s %s", k.Format("2006-01-02 15:04:05"), v.Result, v.FileName, v.Detail)
	}
	_, _ = k.Backend.Send(c.Sender(), msg)
	return nil
}

func (k *Bot) StartHandler(c tele.Context) error {
	_, _ = k.Backend.Send(c.Sender(), "pls set kindle email")
	setUserState(c.Sender().ID, OnSetEmail)
	return nil
}

func (k *Bot) SendFileHandler(c tele.Context) error {
	doc := c.Message().Document
	// check data is in kindle supported format: mobi, epub, azw3, pdf
	if doc.InCloud() {
		var fileName, fileType string
		fileName, fileType = doc.FileName, ""
		sl := strings.Split(fileName, ".")
		if sl == nil || len(sl) < 2 {
			_, _ = k.Backend.Send(c.Sender(), "file format not supported")
			return nil
		} else {
			fileType = sl[len(sl)-1]
			if fileType != "mobi" && fileType != "epub" && fileType != "azw3" && fileType != "pdf" {
				_, _ = k.Backend.Send(c.Sender(), fmt.Sprintf("file type %s not supported", fileType))
				return nil
			}
		}
		file, err := k.Backend.File(&doc.File)
		if err != nil {
			log.Errorf("get file from tg error:%v, file_id:%v", err, doc.FileID)
			return nil
		}
		// split file name
		sl = strings.Split(fileName, "_")
		if len(sl) > 1 {
			fileName = sl[0] + "." + fileType
		}
		if err = k.SendAttachment(fileName, file); err != nil {
			log.Errorf("send file to kindle error:%v, file_id:%v", err, doc.FileID)
			_, _ = k.Backend.Send(c.Sender(), "send file to kindle error")
			if k.Cfg.DebugTrace {
				_, _ = k.Backend.Send(c.Sender(), fmt.Sprintf("send_cloud err: %s", err.Error()))
			}
		}
	}

	return nil
}

func (k *Bot) SendAttachment(fileName string, fileBody io.Reader) (err error) {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	err = w.WriteField("apiUser", "******")           // MASK_DONE
	err = w.WriteField("apiKey", "******") // MASK_DONE
	err = w.WriteField("to", k.Cfg.KindleEmail)
	err = w.WriteField("from", "service@sendcloud.im")
	err = w.WriteField("fromName", "@KindleBurningBot")
	err = w.WriteField("subject", "Kindle Document")
	err = w.WriteField("html", "Kindle Document")
	fw, err := w.CreateFormFile("attachments", fileName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = io.Copy(fw, fileBody)
	if err != nil {
		fmt.Println(err)
		return err
	}
	w.Close()
	req, err := http.NewRequest("POST", "http://api.sendcloud.net/apiv2/mail/send", buf)
	if err != nil {
		log.Errorf("req create err: %s", err)
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("resp err: %s", err)
		return errors.Wrap(err, "post send cloud error")
	}
	defer resp.Body.Close()
	BodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("resp body read err: %s", err)
		return errors.Wrap(err, "resp body read error")
	}
	defer func() {
		if err != nil {
			log.Errorf("send file to kindle error:%v", err)
			k.writeHistory("failed", err.Error(), fileName)
		} else {
			k.writeHistory("success", "", fileName)
		}
	}()
	log.Infof("resp body: %s", string(BodyByte))
	return nil
}

func (k *Bot) writeHistory(result, detail, fileName string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.History[time.Now()] = history{
		Result:   result,
		Detail:   detail,
		FileName: fileName,
	}
}

func setUserState(userID int64, state string) {
	GlobalUserState.mu.Lock()
	defer GlobalUserState.mu.Unlock()
	GlobalUserState.State[userID] = state
}
