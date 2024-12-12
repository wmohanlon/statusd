package main

import (
	"encoding/json"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// report ifconfig.me/all.json
//    IP address
//    Wireguard address
//    uptime
// ....all to MQTT
//     ....as json!

var knt int
var hostname string

type status struct {
	Ip_address string `json:"ip_address"`
	Uptime     string `json:"uptime"`
	Wg_address string `json:"wg_address"`
	Date       string `json:"date"`
	Hostname   string `json:"hostname"`
}

var curr_status status

var f MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	currentTime := time.Now()
	fmt.Printf("%s: %s: %s\n", currentTime.Format(time.Stamp), msg.Topic(), msg.Payload())
	do_an_update(client)
}

func main() {
	knt = 0
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	hostname, _ = os.Hostname()

	opts := MQTT.NewClientOptions().AddBroker("tcp://willymartini.com:1883")
	opts.SetClientID("statusd-" + hostname)
	opts.SetUsername("wmo")
	opts.SetPassword("blowme")
	opts.SetDefaultPublishHandler(f)
	topic := "statusd/incoming"

	opts.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(topic, 0, f); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to server\n")
	}
	go do_updates(client)
	<-c
}

func do_updates(client MQTT.Client) {

	for {
		do_an_update(client)
		time.Sleep(300 * time.Second)
	}
}

func do_an_update(client MQTT.Client) {

	update_status(&curr_status)
	pubtopic := "statusd/" + hostname
	b, err := json.Marshal(curr_status)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	fmt.Println(string(b))
	client.Publish(pubtopic, 0, true, string(b))
}

func update_status(current *status) {
	current.Ip_address = fetch_ext_ipaddr()
	cmd := exec.Command("uptime")
	out, _ := cmd.CombinedOutput()
	current.Uptime = string(out)
	cmd = exec.Command("ifconfig", "-a")
    out, _ = cmd.CombinedOutput()
    fmt.Println(string(out))
	cmd = exec.Command("ifconfig", "wg0")
	out, _ = cmd.CombinedOutput()
	wg := string(out)
	p := strings.Index(wg, "inet ")
	e := strings.Index(wg, "netmask")
	if e == -1 {
		e = strings.Index(wg, "P-t-P")
	}
	if e == -1 {
		current.Wg_address = "no_wireguard"
	} else {
		current.Wg_address = strings.TrimSpace(wg[p+5 : e])
	}
	current.Hostname, _ = os.Hostname()
	currentTime := time.Now()
	current.Date = currentTime.String()
}

func fetch_ext_ipaddr() string {
	response, err := http.Get("http://ifconfig.me/ip")
	if err != nil {
		fmt.Printf("%s", err)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("%s", err)
		}
		return string(contents)
	}
	return ""
}
