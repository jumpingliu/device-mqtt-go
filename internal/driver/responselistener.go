// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

func startCommandResponseListening() error {

	var scheme = driver.serviceConfig.CustomConfig.ResponseSchema
	var brokerUrl = driver.serviceConfig.CustomConfig.ResponseHost
	var brokerPort = driver.serviceConfig.CustomConfig.ResponsePort
	var secretPath = driver.serviceConfig.CustomConfig.ResponseCredentialsPath
	var mqttClientId = driver.serviceConfig.CustomConfig.ResponseClientId
	var qos = byte(driver.serviceConfig.CustomConfig.ResponseQos)
	var keepAlive = driver.serviceConfig.CustomConfig.ResponseKeepAlive
	var topic = driver.serviceConfig.CustomConfig.ResponseTopic

	credentials, err := GetCredentials(secretPath)
	if err != nil {
		return fmt.Errorf("Unable to get response MQTT credentials for secret path '%s': %s", secretPath, err.Error())
	}

	driver.Logger.Info("Response MQTT credentials loaded")

	uri := &url.URL{
		Scheme: strings.ToLower(scheme),
		Host:   fmt.Sprintf("%s:%d", brokerUrl, brokerPort),
		User:   url.UserPassword(credentials.Username, credentials.Password),
	}

	var client mqtt.Client
	for i := 1; i <= driver.serviceConfig.CustomConfig.ConnEstablishingRetry; i++ {
		client, err = createClient(mqttClientId, uri, keepAlive)
		if err != nil && i == driver.serviceConfig.CustomConfig.ConnEstablishingRetry {
			return err
		} else if err != nil {
			driver.Logger.Error(fmt.Sprintf("Fail to initial conn for command response, %v ", err))
			time.Sleep(time.Duration(driver.serviceConfig.CustomConfig.ConnEstablishingRetry) * time.Second)
			driver.Logger.Warn("Retry to initial conn for command response")
			continue
		}
		break
	}

	defer func() {
		if client.IsConnected() {
			client.Disconnect(5000)
		}
	}()

	token := client.Subscribe(topic, qos, onCommandResponseReceived)
	if token.Wait() && token.Error() != nil {
		driver.Logger.Info(fmt.Sprintf("[Response listener] Stop command response listening. Cause:%v", token.Error()))
		return token.Error()
	}

	driver.Logger.Info("[Response listener] Start command response listening. ")
	select {}
}

func onCommandResponseReceived(client mqtt.Client, message mqtt.Message) {
	var response map[string]interface{}

	json.Unmarshal(message.Payload(), &response)
	uuid, ok := response["uuid"].(string)
	if ok {
		driver.CommandResponses.Store(uuid, string(message.Payload()))
		driver.Logger.Info(fmt.Sprintf("[Response listener] Command response received: topic=%v uuid=%v msg=%v", message.Topic(), uuid, string(message.Payload())))
	} else {
		driver.Logger.Warn(fmt.Sprintf("[Response listener] Command response ignored. No UUID found in the message: topic=%v msg=%v", message.Topic(), string(message.Payload())))
	}

}
