package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/line/line-bot-sdk-go/linebot"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type Entity struct {
	Type int
	Date time.Time
}

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
	slackPath := os.Getenv("SLACK_INCOMING_WEBHOOK_PATH")

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
			resMessage := linebot.NewTextMessage("Beaconイベントキャッチ！")
			switch event.Beacon.Type {
			case linebot.BeaconEventTypeEnter:
				resMessage = linebot.NewTextMessage("来た！")

				k := datastore.NewIncompleteKey(cx, "Entity", nil)
				e := new(Entity)
				e.Type = 1
				e.Date = time.Now()

				if _, err := datastore.Put(cx, k, e); err != nil {
					return c.JSON(http.StatusInternalServerError, "register error")
				}

				sendToSlack(c, slackPath, "出社したよ〜！")
			case linebot.BeaconEventTypeLeave:
				resMessage = linebot.NewTextMessage("去った！")
				sendToSlack(c, slackPath, "退社したよ〜！")
			}
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

func sendToSlack(c echo.Context, path string, text string) (string, error) {
	slackURL := "https://hooks.slack.com"
	slackPath := path
	u, _ := url.ParseRequestURI(slackURL)
	u.Path = slackPath

	urlStr := fmt.Sprintf("%v", u)

	data := url.Values{}
	data.Set("payload", "{\"text\": \""+text+"\", \"link_names\": 1}")

	cx := appengine.NewContext(c.Request())
	client := urlfetch.Client(cx)
	req, _ := http.NewRequest("POST", urlStr, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)

	defer res.Body.Close()

	b, _ := ioutil.ReadAll(res.Body)

	if err != nil {
		return string(b), err
	}
	return string(b), nil
}
