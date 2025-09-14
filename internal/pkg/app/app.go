package app

import (
	tele "gopkg.in/telebot.v4"
	"log/slog"
	"privatkabot/internal/app/config"
	"privatkabot/internal/app/timers"
	"privatkabot/pkg/logger"
	"strconv"
	"strings"
	"time"
)

type App struct {
	log logger.Logger
	cfg *config.Config
	tw  *timers.TimingWheel
	bot *tele.Bot
}

func New() error {
	a := &App{
		log: logger.New(),
		tw:  timers.NewTimingWheel(100*time.Millisecond, 600),
	}

	manager, err := config.New("config.json")
	if err != nil {
		a.log.Fatal("Error loading config", err)
	}
	a.log.Info("Config loaded successfully")

	a.cfg = manager.Get()
	a.log.SetLogLevel(a.cfg.App.LoggerLevel)

	a.bot, err = tele.NewBot(tele.Settings{
		Token:  a.cfg.App.TelegramAPI,
		Poller: &tele.LongPoller{Timeout: 1 * time.Second},
	})
	if err != nil {
		a.log.Error("Failed creating telegram bot", err)
		return err
	}

	a.bot.Handle(tele.OnText, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle(tele.OnGame, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle(tele.OnAudio, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle(tele.OnVideo, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle(tele.OnMedia, func(c tele.Context) error {
		return a.processingMessage(c)
	})

	a.bot.Handle("/ex", func(c tele.Context) error {
		if _, ok := a.cfg.AdminUsers[c.Sender().ID]; !ok {
			a.log.Warn("Unauthorized attempt to use /ex", c.Sender().ID)
			return nil
		}

		words := strings.Fields(c.Text())
		if len(words) < 4 {
			return c.Reply("❌ Использование: /ex add <бот> <интервал в секундах> или /ex del <бот>")
		}

		switch words[1] {
		case "add":
			username := strings.TrimPrefix(words[2], "@")
			duration, err := strconv.Atoi(words[3])
			if err != nil {
				return c.Reply("❌ Неверный формат интервала")
			}

			if err := manager.Update(func(cfg *config.Config) {
				cfg.BotExceptions[username] = time.Duration(duration) * time.Second
			}); err != nil {
				a.log.Error("Failed to update config", err)
			}
		case "del":
			username := strings.TrimPrefix(words[2], "@")
			if err := manager.Update(func(cfg *config.Config) {
				delete(cfg.BotExceptions, username)
			}); err != nil {
				a.log.Error("Failed to update config", err)
			}
		default:
			return c.Reply("❌ Неизвестная команда")
		}

		return c.Reply("Успешно!")
	})

	a.bot.Handle("/delete", func(c tele.Context) error {
		if c.Message().ReplyTo == nil {
			return nil
		}
		targetMessage := c.Message().ReplyTo

		if err := a.bot.Delete(targetMessage); err != nil {
			a.log.Error("Failed to delete target message", err, slog.Int("message_id", targetMessage.ID))
		} else {
			a.log.Info("Target message deleted", slog.Int("message_id", targetMessage.ID))
		}

		if err := a.bot.Delete(c.Message()); err != nil {
			a.log.Error("Failed to delete /delete command message", err, slog.Int("message_id", c.Message().ID))
		}

		return nil
	})

	a.log.Info("Starting bot")
	a.bot.Start()
	return nil
}

func (a *App) processingMessage(c tele.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}

	if msg.Via == nil && msg.Animation == nil {
		return nil
	}

	dur := a.cfg.Duration
	if msg.Via != nil {
		if exceptDur, ok := a.cfg.BotExceptions[msg.Via.Username]; ok {
			if exceptDur == time.Duration(0) {
				return nil
			}
			dur = exceptDur
		}
	}

	a.log.Info("Processing message", slog.String("text", c.Message().Text))
	a.tw.AddTimer(strconv.Itoa(c.Message().ID), dur, false, map[string]any{
		"message": c.Message(),
	}, func(m map[string]any) {
		message, ok := m["message"].(*tele.Message)
		if !ok {
			return
		}

		a.log.Info("Timer triggered for message ID", slog.Int("message_id", message.ID))
		if err := a.bot.Delete(message); err != nil {
			a.log.Error("Failed deleting message", err, slog.Int("message_id", message.ID))
		} else {
			a.log.Info("Message deleted successfully", slog.Int("message_id", message.ID))
		}
	})
	return nil
}
