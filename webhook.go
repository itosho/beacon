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
	"strconv"
)

type AttendanceEntity struct {
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

	utcNow := time.Now().UTC()
	jst := time.FixedZone("Asia/Tokyo", 60*60*9)
	now := utcNow.In(jst)

	for _, event := range received {
		if event.Type == linebot.EventTypeBeacon {
			resMessage := linebot.NewTextMessage("Beaconイベントキャッチ！")
			switch event.Beacon.Type {
			case linebot.BeaconEventTypeEnter:
				resMessage = linebot.NewTextMessage("来た！")

				k := datastore.NewIncompleteKey(cx, "Attendance", nil)
				e := new(AttendanceEntity)
				e.Type = 1
				e.Date = now

				if _, err := datastore.Put(cx, k, e); err != nil {
					return c.JSON(http.StatusInternalServerError, "register error")
				}

				r := datastore.NewQuery("Attendance").
					Filter("Date >=", now.Add(-60*time.Minute)).
					Filter("Date <=", now)
				recentCount, _ := r.Count(cx)

				t := datastore.NewQuery("Attendance").
					Filter("Date >=", now.Add(-480*time.Minute)).
					Filter("Date <=", now)
				todayCount, _ := t.Count(cx)

				if recentCount > 9 {
					sendToSlack(c, slackPath, "仕事中なのにここ1時間で"+strconv.Itoa(count)+"回もLINEを起動しているよ！")
				} else if now.Hour() > 10 && todayCount == 1 {
					sendToSlack(c, slackPath, "もう"+strconv.Itoa(now.Hour())+"時だよ！来るの遅い！")
				} else if now.Hour() <= 10 && todayCount == 1 {
					sendToSlack(c, slackPath, "おはよう！今日も１日頑張ろう！")
				}

			case linebot.BeaconEventTypeLeave:
				resMessage = linebot.NewTextMessage("去った！")

				k := datastore.NewIncompleteKey(cx, "Attendance", nil)
				e := new(AttendanceEntity)
				e.Type = 2
				e.Date = now

				if _, err := datastore.Put(cx, k, e); err != nil {
					return c.JSON(http.StatusInternalServerError, "register error")
				}

				if now.Hour() < 19 {
					sendToSlack(c, slackPath, "あれ？今日は帰るの早いね！")
				} else if now.Hour() >= 22 {
					sendToSlack(c, slackPath, "今日は遅くまでよく頑張りました！")
				} else if now.Hour() >= 19 {
					sendToSlack(c, slackPath, "今日も１日お疲れ様！")
				}
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
