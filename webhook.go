package main

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/line/line-bot-sdk-go/linebot"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	g := e.Group("/webhook")
	g.Use(middleware.CORS())
	g.GET("", getCurrentTime)
	g.POST("", postMessage)
}

func getCurrentTime(c echo.Context) error {
	return c.JSON(http.StatusOK, time.Now().String())
}

func postMessage(c echo.Context) error {
	secret := os.Getenv("CHANNEL_SECRET")
	accessToken := os.Getenv("CHANNEL_ACCESS_TOKEN")

	cx := appengine.NewContext(c.Request())
	client := urlfetch.Client(cx)

	bot, err := linebot.New(secret, accessToken, linebot.WithHTTPClient(client))
	if err != nil {
		log.Infof(cx, err.Error())
		return c.JSON(http.StatusInternalServerError, err)
	}

	received, err := bot.ParseRequest(c.Request())
	if err != nil {
		log.Infof(cx, err.Error())
		return c.JSON(http.StatusInternalServerError, err)
	}

	for _, event := range received {
		if event.Type == linebot.EventTypeBeacon {
			resMessage := linebot.NewTextMessage("beacon!")
			if _, err = bot.ReplyMessage(event.ReplyToken, resMessage).Do(); err != nil {
				log.Errorf(cx, "send error: %v", err)
			}
		}

		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				log.Infof(cx, "TextMessage %#v", message)
				resMessage := linebot.NewTextMessage(message.Text)
				if _, err = bot.ReplyMessage(event.ReplyToken, resMessage).Do(); err != nil {
					log.Errorf(cx, "send error: %v", err)
				}
			}
		}
	}
	return c.JSON(http.StatusOK, "success")
}
