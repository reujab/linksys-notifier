package main

import (
	"bytes"
	"flag"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"text/template"
	"time"

	nma "github.com/dustin/go-nma"
	pushbullet "github.com/mitsuse/pushbullet-go"
	"github.com/mitsuse/pushbullet-go/requests"
	"github.com/reujab/linksys"
)

type notificationInfo struct {
	Hostname  string
	Connected bool
	IP        string
	MAC       string
}

var (
	testNotification     = flag.Bool("test-notification", false, "send a test notification and exit")
	updateInterval       = flag.Int("update-interval", 1, "the update interval in seconds")
	shellScript          = flag.String("shell-script", "", "the path of a shellscript to execute on new connections")
	notificationTemplate = flag.String("notification", `{{.Hostname}} {{if .Connected}}connected{{else}}disconnected{{end}}.`, "the notification template")
	pushoverApp          = flag.String("pushover-app", "", "your pushover application API token")
	pushoverUser         = flag.String("pushover-user", "", "your pushover user key")
	pushbulletToken      = flag.String("pushbullet", "", "your pushbullet access token")
	nmaToken             = flag.String("nma", "", "your notify my android access token")
)

func main() {
	flag.Parse()
	if *testNotification {
		sendNotification(notificationInfo{
			Hostname:  "Router",
			Connected: true,
			IP:        "192.168.1.1",
			MAC:       "AA:BB:CC:DD:EE:FF",
		})
		return
	}

	client := linksys.NewClient()
	devices, err := client.GetDevices(0)
	die(err)

	revision := devices.Revision
	var connectedDevices []string
	for _, device := range devices.Devices {
		if len(device.Connections) != 0 {
			connectedDevices = append(connectedDevices, device.GUID)
		}
	}

	for {
		time.Sleep(time.Second * time.Duration(*updateInterval))

		devices, err = client.GetDevices(revision)
		die(err)
		if devices.Revision == revision {
			continue
		}
		revision = devices.Revision

	deviceLoop:
		for _, device := range devices.Devices {
			if len(device.Connections) == 0 || device.Connections[0].IP == "" {
				for i, guid := range connectedDevices {
					if guid == device.GUID {
						connectedDevices = append(connectedDevices[:i], connectedDevices[i+1:]...)
						sendNotification(notificationInfo{
							Hostname:  device.Hostname,
							Connected: false,
						})
						break
					}
				}
			} else {
				for _, guid := range connectedDevices {
					if guid == device.GUID {
						break deviceLoop
					}
				}

				connectedDevices = append(connectedDevices, device.GUID)
				sendNotification(notificationInfo{
					Hostname:  device.Hostname,
					Connected: true,
					IP:        device.Connections[0].IP,
					MAC:       device.MACAddresses[0],
				})
			}
		}
	}
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}

func sendNotification(notificationInfo notificationInfo) {
	buffer := bytes.NewBufferString("")
	err := template.Must(template.New("notification").Parse(*notificationTemplate)).Execute(buffer, notificationInfo)
	if err != nil {
		panic(err)
	}
	notification := buffer.String()

	if *shellScript != "" {
		cmd := exec.Command(*shellScript, notificationInfo.Hostname, strconv.FormatBool(notificationInfo.Connected), notificationInfo.IP, notificationInfo.MAC)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	if *pushoverApp != "" && *pushoverUser != "" {
		values := url.Values{}
		values.Set("token", *pushoverApp)
		values.Set("user", *pushoverUser)
		values.Set("message", notification)
		_, err = http.PostForm("https://api.pushover.net/1/messages.json", values)
		if err != nil {
			panic(err)
		}
	}

	if *pushbulletToken != "" {
		pb := pushbullet.New(*pushbulletToken)
		note := requests.NewNote()
		note.Title = "Router"
		note.Body = notification
		_, err = pb.PostPushesNote(note)
		if err != nil {
			panic(err)
		}
	}

	if *nmaToken != "" {
		app := nma.New(*nmaToken)
		err = app.Notify(&nma.Notification{
			Application: "Router",
			Description: notification,
		})
		if err != nil {
			panic(err)
		}
	}
}
