// Implementation of the MQTT Client
package main

//import (
//	"encoding/json"
//	"fmt"
//	"log"
//	"time"
//
//	"code.linksmart.eu/dt/deployment-tool/model"
//	"github.com/eclipse/paho.mqtt.golang"
//)
//
//func StartMQTTClient(uri string) {
//	opts := mqtt.NewClientOptions()
//	opts.AddBroker(uri)
//
//	c := mqtt.NewClient(opts)
//	if token := c.Connect(); token.Wait() && token.Error() != nil {
//		log.Fatal(token.Error())
//	}
//
//	if token := c.Subscribe("ids/command", 1, mqttMessageHandler); token.Wait() && token.Error() != nil {
//		log.Fatal(token.Error())
//	}
//}
//
//func mqttMessageHandler(client mqtt.Client, msg mqtt.Message) {
//	fmt.Printf("TOPIC: %s\n", msg.Topic())
//	fmt.Printf("MSG: %s\n", msg.Payload())
//
//	var task model.Task
//	err := json.Unmarshal(msg.Payload(), &task)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	batchRes := make(chan model.BatchResponse)
//	go func() {
//		for x := range batchRes {
//			log.Printf("Batch: %+v", x)
//			resp, err := json.Marshal(x)
//			if err != nil {
//				log.Fatal(err)
//			}
//			client.Publish("ids/resp", 1, false, resp)
//		}
//	}()
//
//	responseBatchCollector(task.Commands, time.Duration(3)*time.Second, batchRes)
//	log.Println("end")
//
//}
