package internal

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

type Config struct {
	TelegramToken  string `yaml:"bot_token"`
	TelegramUserID string `yaml:"bot_userid"`
	MinioURL       string `yaml:"minio_endpoint"`
	MinioAccessKey string `yaml:"minio_access_key"`
	MinioSecretKey string `yaml:"minio_secret_key"`
	BucketName     string `yaml:"bucket_name"`
}

var cfg *Config

func init() {
	if cfg == nil {
		cfg = &Config{
			TelegramToken:  "",
			TelegramUserID: "",
			MinioURL:       "",
			MinioAccessKey: "",
			MinioSecretKey: "",
			BucketName:     "",
		}
	}

	// log
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func Server() {
	b, err := tb.NewBot(tb.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Errorf("init bot error:%v", err)
	}

	b.Handle(tb.OnDocument, func(m tb.Context) error {
		userid, _ := strconv.ParseUint(cfg.TelegramUserID, 10, 64)
		if m.Sender().ID == int64(userid) {
			doc := m.Message().Document
			if doc.InCloud() {
				file, err := b.File(&doc.File)
				if err != nil {
					log.Errorf("get file from tg error:%v, file_id:%v", err, doc.FileID)
					return nil
				}
				// upload file
				ctx := context.Background()
				mc, err := minio.New(cfg.MinioURL, &minio.Options{
					Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
					Secure: true,
				})
				if err != nil {
					log.Errorf("init s3 client error:%v", err)
					b.Send(m.Sender(), "init s3 client error :(")
					return err
				}
				info, err := mc.PutObject(ctx, cfg.BucketName, doc.FileName, file, int64(doc.FileSize), minio.PutObjectOptions{})
				if err != nil {
					log.Fatalln(err)
				}
				b.Send(m.Sender(), "upload success")
				reqParams := make(url.Values)
				reqParams.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", doc.FileName))
				url, err := mc.PresignedGetObject(ctx, cfg.BucketName, doc.FileName, time.Hour, reqParams)
				if err != nil {
					b.Send(m.Sender(), "maybe s3 server error, pls wait or check")
				}
				log.Printf("successfully uploaded %s of size %d\n", doc.FileName, info.Size)
				b.Send(m.Sender(), url.String())
			}
		}
		return nil
	})
	b.Start()
}
