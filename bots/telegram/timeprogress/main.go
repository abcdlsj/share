package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
	tele "gopkg.in/telebot.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TrackItem struct {
	ID               uint64 `gorm:"primaryKey"`
	Name             string `gorm:"column:name"`
	TimeStart        string `gorm:"column:time_start"`
	TimeEnd          string `gorm:"column:time_end"`
	AnnounceInterval int    `gorm:"column:announce_interval"`
	LastAnnounceTime string `gorm:"column:last_announce_time"`
}

var (
	filepath              = orEnv("DB_FILE", "timeprogress.db")
	db                    = initDB(filepath)
	announceIntervalCheck = orEnv("ANNOUNCE_INTERVAL_CHECK", "true")

	PASTCELL   = "▓"
	FUTURECELL = "░"

	cronRunning = false
)

func initDB(filepath string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(filepath), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = db.AutoMigrate(&TrackItem{})
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func main() {
	bot, err := tele.NewBot(tele.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("new bot error: %v", err)
	}

	bot.Handle("/start", func(c tele.Context) error {
		cr := cron.New()

		cr.AddFunc("@every 5m", func() {
			var items []TrackItem

			db.Find(&items)

			needSends := make([]TrackItem, 0)

			for _, item := range items {
				start, _ := time.Parse("20060102", item.TimeStart)
				end, _ := time.Parse("20060102", item.TimeEnd)
				lastAnnounceTime, _ := time.Parse("2006-01-02 15:04:05", item.LastAnnounceTime)
				now := time.Now()

				if end.Before(now) || start.After(now) {
					continue
				}

				if announceIntervalCheck == "true" &&
					now.Sub(lastAnnounceTime).Hours() < float64(item.AnnounceInterval) {
					continue
				}

				needSends = append(needSends, item)

				db.Model(&item).Update("last_announce_time", now.Format("2006-01-02 15:04:05"))
			}

			for _, item := range needSends {
				start, _ := time.Parse("20060102", item.TimeStart)
				end, _ := time.Parse("20060102", item.TimeEnd)
				now := time.Now()

				bar := renderProgress(start, end, now)

				bot.Send(c.Sender(), renderMd(item.Name+"\n"+bar))

				time.Sleep(1 * time.Minute)
			}
		})

		cr.Start()
		cronRunning = true

		return c.Send("started")
	})

	bot.Handle("/add", func(c tele.Context) error {
		args := c.Args()

		if len(args) != 4 {
			return c.Send("pls use /add name time_start time_end announce_interval")
		}

		interval, _ := strconv.Atoi(args[3])

		item := TrackItem{
			Name:             args[0],
			TimeStart:        args[1],
			TimeEnd:          args[2],
			AnnounceInterval: interval,
			LastAnnounceTime: time.Now().Format("2006-01-02 15:04:05"),
		}

		db.Create(&item)

		return c.Send("add ok")
	})

	bot.Handle("/delete", func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send("pls use /delete name")
		}

		db.Delete(&TrackItem{}, "name = ?", args[0])

		return c.Send("delete ok")
	})

	bot.Handle("/list", func(c tele.Context) error {
		var items []TrackItem
		db.Find(&items)

		ret := "all tracking items:\n"

		for _, item := range items {
			end, _ := time.Parse("20060102", item.TimeEnd)
			if end.Before(time.Now()) {
				continue
			}

			ret += fmt.Sprintf("- %s, start: %s, end: %s, announce interval: %d, last announce time: %s\n", item.Name, item.TimeStart, item.TimeEnd, item.AnnounceInterval, item.LastAnnounceTime)
		}

		return c.Send(ret)
	})

	bot.Handle("/status", func(c tele.Context) error {
		return c.Send("cron running: " + strconv.FormatBool(cronRunning))
	})

	bot.Start()
}

func orEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func renderMd(text string) string {
	return text
}

func renderProgress(start, end time.Time, now time.Time) string {
	percent := float64(now.Unix()-start.Unix()) / float64(end.Unix()-start.Unix())

	// generate 20 cells
	pastNum := int(percent / float64(0.05))
	futureNum := 20 - pastNum

	return strings.Repeat(PASTCELL, pastNum) + strings.Repeat(FUTURECELL, futureNum) + " " + strconv.Itoa(int(percent*100)) + "%"
}
