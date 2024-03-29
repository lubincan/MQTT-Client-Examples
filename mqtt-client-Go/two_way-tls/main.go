package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ConConfig struct {
	Broker   string
	Topic    string
	Username string
	Password string
	Cafile   string
	Cert     string
	Key      string
}

var ExitFlag bool = false

func main() {
	host := flag.String("host", "127.0.0.1", "server hostname or IP")
	port := flag.Int("port", 8883, "server port")
	topic := flag.String("topic", "golang-mqtt/test", "publish/subscribe topic")
	username := flag.String("username", "emqx", "username")
	password := flag.String("password", "public", "password")
	cafile := flag.String("cafile", "",
		"path to a file containing trusted CA certificates to enable encryptedommunication.")
	cert := flag.String("cert", "",
		"client certificate for authentication, if required by server.")
	key := flag.String("key", "",
		"client certificate for authentication, if required by server.")

	flag.Parse()

	config := ConConfig{
		Broker:   fmt.Sprintf("tls://%s:%d", *host, *port),
		Topic:    *topic,
		Username: *username,
		Password: *password,
		Cafile:   *cafile,
		Cert:     *cert,
		Key:      *key,
	}
	client := mqttConnect(&config)
	go sub(client, &config)
	publish(client, &config)
}

func publish(client mqtt.Client, config *ConConfig) {
	for !ExitFlag {
		payload := "The current time " + time.Now().String()
		if client.IsConnectionOpen() {
			token := client.Publish(config.Topic, 0, false, payload)
			if token.Error() != nil {
				log.Printf("pub message to topic %s error:%s \n", config.Topic, token.Error())
			} else {
				log.Printf("pub %s to topic [%s]\n", payload, config.Topic)
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func sub(client mqtt.Client, config *ConConfig) {
	token := client.Subscribe(config.Topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("sub [%s] %s\n", msg.Topic(), string(msg.Payload()))
	})
	if token.Error() != nil {
		log.Printf("sub to message error from topic:%s \n", config.Topic)
	}
	ack := token.WaitTimeout(3 * time.Second)
	if !ack {
		log.Printf("sub to topic timeout: %s \n", config.Topic)
	}
}

func SetAutoReconnect(config *ConConfig, opts *mqtt.ClientOptions) {
	firstReconnectDelay, maxReconnectDelay, maxReconnectCount, reconnectRate := 1, 60, 12, 2

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		sub(client, config)
		log.Println("Connected to MQTT Broker!")
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v\nTrying to reconnect...\n", err)

		reconnectDelay := firstReconnectDelay
		for i := 0; i < maxReconnectCount; i++ {
			log.Printf("Reconnecting in %ds.\n", reconnectDelay)
			time.Sleep(time.Duration(reconnectDelay) * time.Second)
			if token := client.Connect(); token.Wait() && token.Error() != nil {
				log.Printf("Failed to reconnect: %v\n", token.Error())
			} else if client.IsConnectionOpen() {
				return
			}
			if i != maxReconnectCount-1 {
				log.Println("Reconnect failed, waiting for the next reconnection.")
			}
			reconnectDelay *= reconnectRate
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
		}
		log.Printf("Reconnect failed after %d attempts. Exiting...", maxReconnectCount)
		ExitFlag = !client.IsConnectionOpen()
	})
}

func mqttConnect(config *ConConfig) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.TLSConfig = loadTLSConfig(config)
	opts.SetKeepAlive(3 * time.Second)
	SetAutoReconnect(config, opts)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	ack := token.WaitTimeout(5 * time.Second)
	if token.Error() != nil || !ack {
		log.Fatalf("connect%s mqtt server error: %s", config.Broker, token.Error())
	}
	return client
}

func loadTLSConfig(config *ConConfig) *tls.Config {
	// load tls config

	var tlsConfig tls.Config
	tlsConfig.InsecureSkipVerify = false
	if config.Cafile != "" {
		certpool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(config.Cafile)
		if err != nil {
			log.Fatalln(err.Error())
		}
		certpool.AppendCertsFromPEM(ca)
		tlsConfig.RootCAs = certpool
	}
	if config.Cert != "" && config.Key != "" {
		clientKeyPair, err := tls.LoadX509KeyPair(config.Cert, config.Key)
		if err != nil {
			log.Fatalln(err.Error())
		}
		tlsConfig.ClientAuth = tls.RequestClientCert
		tlsConfig.Certificates = []tls.Certificate{clientKeyPair}
	}
	return &tlsConfig
}
