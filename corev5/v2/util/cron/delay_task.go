package cron

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/logger"
	"time"
)

var DelayTaskManager = NewMemDelayTaskManage()

type ID = string

type DelayWillTask struct {
	ID       ID
	DealTime time.Duration // 处理时间
	Data     *message.PublishMessage
	Fn       func(data *message.PublishMessage)

	icron Icron
}

func (g *DelayWillTask) Run() {
	defer func() {
		if err := recover(); err != nil {
			logger.Logger.Error(err)
		}
		g.icron.Remove(g.ID)
	}()
	g.Fn(g.Data)
}

type DelaySessionTask struct {
	ID       ID
	DealTime time.Duration // 处理时间
	Data     sessions.Session
	Fn       func(data sessions.Session)

	icron Icron
}

func (g *DelaySessionTask) Run() {
	defer func() {
		if err := recover(); err != nil {
			logger.Logger.Error(err)
		}
		g.icron.Remove(g.ID)
	}()
	g.Fn(g.Data)
}

type DelayTaskManage interface {
	DelaySendWill(*DelayWillTask) error
	DelayCleanSession(*DelaySessionTask) error
	Cancel(ID)
}

type memDelayTaskManage struct {
	icron Icron
}

func NewMemDelayTaskManage() DelayTaskManage {
	return &memDelayTaskManage{icron: Get()}
}
func (d *memDelayTaskManage) DelaySendWill(task *DelayWillTask) error {
	logger.Logger.Debugf("添加%s的延迟发送任务, 延迟时间：%ds", task.ID, task.DealTime)
	if task.DealTime <= 0 {
		//task.DealTime = 1
		go func() {
			task.Fn(task.Data)
		}()
	}
	task.icron = d.icron

	err := d.icron.AddJob(fmt.Sprintf("@every %ds", task.DealTime), task.ID, task)
	return err
}

func (d *memDelayTaskManage) DelayCleanSession(task *DelaySessionTask) error {
	logger.Logger.Debugf("添加%s的延迟发送任务, 延迟时间：%ds", task.ID, task.DealTime)
	if task.DealTime <= 0 {
		//task.DealTime = 1
		go func() {
			task.Fn(task.Data)
		}()
	}
	err := d.icron.AddJob(fmt.Sprintf("@every %ds", task.DealTime), task.ID, task)
	return err
}

func (d *memDelayTaskManage) Cancel(id ID) {
	logger.Logger.Debugf("取消%s的延迟发送任务", id)
	d.icron.Remove(id)
}
